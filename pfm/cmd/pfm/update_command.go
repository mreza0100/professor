package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"hostops/pfm/internal/atomicfile"
	"hostops/pfm/internal/deps"
	"hostops/pfm/internal/installer"
)

// These seams keep update tests entirely inside their throwaway repositories;
// production uses the real build/install/doctor functions below.
var (
	updateBuildCandidate  = buildUpdateCandidate
	updateApplyInstall    = applyUpdateInstall
	updateRunDoctor       = runUpdateDoctor
	updateRollbackInstall = applyUpdateInstall
	updateRollbackDoctor  = runUpdateDoctor
)

type updateVersion struct {
	major int
	minor int
	patch int
}

func (version updateVersion) less(other updateVersion) bool {
	if version.major != other.major {
		return version.major < other.major
	}
	if version.minor != other.minor {
		return version.minor < other.minor
	}
	return version.patch < other.patch
}

func parseUpdateVersion(tag string) (updateVersion, bool) {
	parts := strings.Split(strings.TrimSpace(tag), ".")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "v") {
		return updateVersion{}, false
	}
	major, err := strconv.Atoi(strings.TrimPrefix(parts[0], "v"))
	if err != nil || major < 0 {
		return updateVersion{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return updateVersion{}, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return updateVersion{}, false
	}
	return updateVersion{major: major, minor: minor, patch: patch}, true
}

func selectHighestSemver(tags []string) (string, error) {
	ordered := append([]string(nil), tags...)
	sort.Strings(ordered)
	var selected string
	var selectedVersion updateVersion
	for _, tag := range ordered {
		version, ok := parseUpdateVersion(tag)
		if !ok {
			continue
		}
		if selected == "" || selectedVersion.less(version) {
			selected, selectedVersion = tag, version
		}
	}
	if selected == "" {
		return "", errors.New("no semantic-version tags (expected vMAJOR.MINOR.PATCH)")
	}
	return selected, nil
}

func runUpdate(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	if len(args) > 0 {
		switch args[0] {
		case "check", "adopt", "pin", "ignore", "drop":
			runtime, err := optionalCommandRuntime(runtimes)
			if err != nil {
				fmt.Fprintf(stderr, "pfm update: config: %v\n", err)
				return 1
			}
			return runProjectUpdate(args[0], args[1:], stdout, stderr, runtime)
		}
	}
	flags := newFlagSet(
		"update",
		"usage: pfm update [--to vX.Y.Z] [--repo PATH] [--skip-harvest] [--root DIR] [--json]\n       pfm update {check|adopt|pin|ignore|drop} [options]",
		stderr,
	)
	target := flags.String("to", "", "target semantic-version tag")
	repoFlag := flags.String("repo", "", "source clone to update")
	skipHarvest := flags.Bool("skip-harvest", false, "leave the optional harvestpy runtime unmanaged")
	projectRoot := flags.String("root", "", "project root used for the post-update template report")
	jsonOutput := flags.Bool("json", false, "write the project report as one JSON object")
	positional, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(positional) != 0 {
		flags.Usage()
		return 2
	}
	runtime, err := optionalCommandRuntime(runtimes)
	if err != nil {
		fmt.Fprintf(stderr, "pfm update: config: %v\n", err)
		return 1
	}
	repo := strings.TrimSpace(*repoFlag)
	if repo == "" {
		repo, err = installer.ReadSourceRepoMarker(runtime.Paths.Home)
		if err != nil {
			fmt.Fprintf(stderr, "pfm update: %v\n", err)
			return 1
		}
	}
	repo, err = filepath.Abs(repo)
	if err != nil {
		fmt.Fprintf(stderr, "pfm update: resolve repository: %v\n", err)
		return 1
	}
	if err := updateRepository(context.Background(), repo, *target, *skipHarvest, stdout, stderr, runtime); err != nil {
		fmt.Fprintf(stderr, "pfm update: %v\n", err)
		return 1
	}
	root, found, err := resolveProjectRoot(*projectRoot)
	if err != nil {
		writeProjectFailure(stdout, *jsonOutput, err)
		return 1
	}
	if found {
		return renderProjectCheck(root, runtime.Paths.Home, *jsonOutput, stdout)
	}
	writeProjectUnmanaged(stdout, *jsonOutput)
	return 0
}

func updateRepository(
	ctx context.Context,
	repo, requestedTag string,
	skipHarvest bool,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) (err error) {
	previousRef, err := updateGitOutput(ctx, repo, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return fmt.Errorf("resolve current revision: %w", err)
	}
	previousRef = strings.TrimSpace(previousRef)
	if _, err := updateGitOutput(ctx, repo, "symbolic-ref", "--quiet", "--short", "HEAD"); err != nil {
		return errors.New("source checkout is detached; checkout its update branch before running pfm update")
	}

	if err := updateGitRun(ctx, repo, "fetch", "--tags"); err != nil {
		return fmt.Errorf("fetch tags: %w", err)
	}
	tagOutput, err := updateGitOutput(ctx, repo, "tag", "--list")
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}
	tags := strings.Fields(tagOutput)
	target := strings.TrimSpace(requestedTag)
	if target == "" {
		target, err = selectHighestSemver(tags)
		if err != nil {
			return fmt.Errorf("resolve latest release: %w", err)
		}
	} else if _, ok := parseUpdateVersion(target); !ok {
		return fmt.Errorf("invalid target tag %q (expected vMAJOR.MINOR.PATCH)", target)
	}
	if !containsString(tags, target) {
		return fmt.Errorf("target tag %q is not present after fetch", target)
	}
	status, err := updateGitOutput(ctx, repo, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("inspect worktree: %w", err)
	}
	if strings.TrimSpace(status) != "" {
		return errors.New("refuse dirty worktree; commit or stash changes before update")
	}
	sourceAlreadyContainsTarget := updateGitRun(ctx, repo, "merge-base", "--is-ancestor", target, previousRef) == nil
	if !sourceAlreadyContainsTarget {
		if err := updateGitRun(ctx, repo, "merge-base", "--is-ancestor", previousRef, target); err != nil {
			return fmt.Errorf("target %s does not fast-forward the current source branch", target)
		}
	} else {
		currentTag, describeErr := updateGitOutput(ctx, repo, "describe", "--tags", "--abbrev=0", previousRef)
		currentVersion, currentOK := parseUpdateVersion(strings.TrimSpace(currentTag))
		targetVersion, targetOK := parseUpdateVersion(target)
		if describeErr == nil && currentOK && targetOK && targetVersion.less(currentVersion) {
			return fmt.Errorf("target %s would downgrade source from %s", target, strings.TrimSpace(currentTag))
		}
	}

	managedRoot := filepath.Dir(installer.SourceRepoPath(runtime.Paths.Home))
	stage, err := os.MkdirTemp(filepath.Dir(managedRoot), "update-")
	if err != nil {
		return fmt.Errorf("stage update beside managed root: %w", err)
	}
	worktreeAdded := false
	defer func() {
		var cleanupErr error
		if worktreeAdded {
			if removeErr := updateGitRun(ctx, repo, "worktree", "remove", "--force", filepath.Join(stage, "source")); removeErr != nil {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("cleanup staged source worktree: %w", removeErr))
			}
		}
		if removeErr := os.RemoveAll(stage); removeErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("cleanup update stage %s: %w", stage, removeErr))
		}
		if cleanupErr == nil {
			return
		}
		if err != nil {
			err = errors.Join(err, cleanupErr)
			return
		}
		fmt.Fprintf(stderr, "pfm update: cleanup warning after successful update: %v\n", cleanupErr)
	}()
	stagedSource := filepath.Join(stage, "source")
	if err := updateGitRun(ctx, repo, "worktree", "add", "--detach", "--quiet", stagedSource, target); err != nil {
		return fmt.Errorf("stage source at %s: %w", target, err)
	}
	worktreeAdded = true
	candidateA := filepath.Join(stage, "pfm-a")
	candidateB := filepath.Join(stage, "pfm-b")
	if err := updateBuildCandidate(ctx, stagedSource, target, candidateA); err != nil {
		return fmt.Errorf("build candidate first pass: %w", err)
	}
	if err := updateBuildCandidate(ctx, stagedSource, target, candidateB); err != nil {
		return fmt.Errorf("build candidate second pass: %w", err)
	}
	hashA, err := fileHash(candidateA)
	if err != nil {
		return fmt.Errorf("hash first candidate: %w", err)
	}
	hashB, err := fileHash(candidateB)
	if err != nil {
		return fmt.Errorf("hash second candidate: %w", err)
	}
	if hashA != hashB {
		return fmt.Errorf("candidate hash mismatch: first=%s second=%s", hashA, hashB)
	}
	if err := updateGitRun(ctx, repo, "worktree", "remove", "--force", stagedSource); err != nil {
		return fmt.Errorf("remove staged source worktree: %w", err)
	}
	worktreeAdded = false

	ledger, err := installer.ReadBinaryOwnership(runtime.Paths.Home)
	if err != nil {
		return err
	}
	if len(ledger.Paths) == 0 {
		return errors.New("binary ownership ledger is empty; refusing to overwrite PATH copies")
	}
	installSourceRepo := preferredUpdateSourceRepo(runtime.Paths.Home, repo)
	replacements := make([]updateReplacement, 0, len(ledger.Paths))
	for _, targetPath := range ledger.Paths {
		if strings.TrimSpace(targetPath) == "" || !filepath.IsAbs(targetPath) {
			return fmt.Errorf("binary ownership ledger contains invalid path %q", targetPath)
		}
		backup := filepath.Join(stage, fmt.Sprintf("previous-%d", len(replacements)))
		if err := copyUpdateFile(targetPath, backup); err != nil {
			return fmt.Errorf("preserve owned binary %s: %w", targetPath, err)
		}
		replacements = append(replacements, updateReplacement{target: targetPath, backup: backup})
	}
	hookSnapshots, err := snapshotUpdateHookFiles(runtime)
	if err != nil {
		return fmt.Errorf("snapshot hook files before install: %w", err)
	}
	sourceAdvanced := false
	if !sourceAlreadyContainsTarget {
		if err := updateGitRun(ctx, repo, "merge", "--ff-only", "--quiet", target); err != nil {
			return fmt.Errorf("fast-forward source branch to %s: %w", target, err)
		}
		sourceAdvanced = true
	}
	for index := range replacements {
		if err := replaceUpdateFile(candidateA, replacements[index].target); err != nil {
			rollbackErr := rollbackUpdateState(
				ctx, repo, installSourceRepo, previousRef, sourceAdvanced, replacements, nil, runtime, skipHarvest, stdout, stderr,
			)
			return updateFailure(fmt.Errorf("replace owned binary %s: %w", replacements[index].target, err), rollbackErr)
		}
		replacements[index].replaced = true
	}

	installErr := updateApplyInstall(ctx, candidateA, repo, installSourceRepo, runtime, skipHarvest, stdout, stderr)
	recordUpdateHookAfter(hookSnapshots)
	if installErr != nil {
		return updateFailure(
			fmt.Errorf("install --yes after staging: %w", installErr),
			rollbackUpdateState(ctx, repo, installSourceRepo, previousRef, sourceAdvanced, replacements, hookSnapshots, runtime, skipHarvest, stdout, stderr),
		)
	}
	if err := updateRunDoctor(ctx, candidateA, runtime, skipHarvest, stdout, stderr); err != nil {
		return updateFailure(
			fmt.Errorf("doctor after update: %w", err),
			rollbackUpdateState(ctx, repo, installSourceRepo, previousRef, sourceAdvanced, replacements, hookSnapshots, runtime, skipHarvest, stdout, stderr),
		)
	}
	fmt.Fprintf(stdout, "updated %s from %s\n", target, repo)
	return nil
}

type updateReplacement struct {
	target   string
	backup   string
	replaced bool
}

func updateFailure(primary, rollbackErr error) error {
	if rollbackErr != nil {
		return fmt.Errorf("%w; rollback residue: %v; manually repair the reported update-owned state", primary, rollbackErr)
	}
	return fmt.Errorf("%w; rolled back update-owned changes", primary)
}

func rollbackUpdateReplacements(replacements []updateReplacement, stderr io.Writer) error {
	var rollbackErr error
	for index := len(replacements) - 1; index >= 0; index-- {
		replacement := replacements[index]
		if !replacement.replaced {
			continue
		}
		if err := replaceUpdateFile(replacement.backup, replacement.target); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("%s: %w", replacement.target, err))
			continue
		}
		fmt.Fprintf(stderr, "pfm update: rolled back %s\n", replacement.target)
	}
	return rollbackErr
}

// rollbackUpdateState first restores every owned binary and the source, then
// the hook files the candidate's install rewrote, and only then uses the prior
// binary's embedded installer to converge installer-owned host wiring back to
// the previous release. The hook restore has to come first: that installer
// recognises only hooks IT generates, so a hook only the newer release knows
// would survive it. A clean doctor is part of rollback proof; without it,
// updateFailure reports residue instead of claiming a safe rollback.
func rollbackUpdateState(
	ctx context.Context,
	repo, installSourceRepo, previousRef string,
	sourceAdvanced bool,
	replacements []updateReplacement,
	hookSnapshots []updateHookSnapshot,
	runtime commandRuntime,
	skipHarvest bool,
	stdout, stderr io.Writer,
) error {
	rollbackErr := rollbackUpdateReplacements(replacements, stderr)
	if sourceAdvanced {
		if err := updateGitRun(ctx, repo, "reset", "--keep", previousRef); err != nil {
			return errors.Join(rollbackErr, fmt.Errorf("restore source revision %s: %w", previousRef, err))
		}
		fmt.Fprintf(stderr, "pfm update: rolled back source to %s\n", previousRef)
	}
	rollbackErr = errors.Join(rollbackErr, restoreUpdateHookFiles(hookSnapshots, stderr))
	if len(replacements) == 0 {
		return errors.Join(rollbackErr, errors.New("no previous binary is available to restore installer state"))
	}
	previousBinary := replacements[0].backup
	if err := updateRollbackInstall(ctx, previousBinary, repo, installSourceRepo, runtime, skipHarvest, stdout, stderr); err != nil {
		return errors.Join(rollbackErr, fmt.Errorf("reapply previous installer state: %w", err))
	}
	if err := updateRollbackDoctor(ctx, previousBinary, runtime, skipHarvest, stdout, stderr); err != nil {
		return errors.Join(rollbackErr, fmt.Errorf("doctor after rollback: %w", err))
	}
	return rollbackErr
}

// updateHookSnapshot is one hook-bearing file captured around the candidate's
// `install --yes`: its bytes before (the state rollback returns to) and right
// after (the only state rollback may overwrite).
type updateHookSnapshot struct {
	path          string // physical path: a symlinked account settings file is written through, never replaced
	before        []byte
	beforeExisted bool
	beforeMode    fs.FileMode
	after         []byte
	afterExisted  bool
	afterErr      error
}

// snapshotUpdateHookFiles captures every file whose hooks the installer owns
// (installer.ExpectedHooks: each account's Claude settings, each Codex hooks
// file) plus the ownership ledger they reconcile against. That is the class
// whose rollback residue is acute — a hook naming a subcommand only the newer
// release implements runs on every prompt against the restored binary.
func snapshotUpdateHookFiles(runtime commandRuntime) ([]updateHookSnapshot, error) {
	home := runtime.Paths.Home
	candidates := []string{filepath.Join(filepath.Dir(installer.SourceRepoPath(home)), "settings-hook-ownership.json")}
	for _, hook := range installer.ExpectedHooks(home, runtime.Config) {
		candidates = append(candidates, hook.File)
	}
	seen := make(map[string]bool, len(candidates))
	snapshots := make([]updateHookSnapshot, 0, len(candidates))
	for _, candidate := range candidates {
		physical, err := filepath.EvalSymlinks(candidate)
		if errors.Is(err, fs.ErrNotExist) {
			physical = filepath.Clean(candidate)
		} else if err != nil {
			return nil, fmt.Errorf("resolve hook file %s: %w", candidate, err)
		}
		if seen[physical] {
			continue
		}
		seen[physical] = true
		content, mode, existed, err := readUpdateHookFile(physical)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, updateHookSnapshot{path: physical, before: content, beforeExisted: existed, beforeMode: mode})
	}
	sort.Slice(snapshots, func(left, right int) bool { return snapshots[left].path < snapshots[right].path })
	return snapshots, nil
}

// recordUpdateHookAfter captures each file exactly as the candidate's install
// left it. A file that cannot be read keeps its error, and restore then
// refuses to touch it.
func recordUpdateHookAfter(snapshots []updateHookSnapshot) {
	for index := range snapshots {
		snapshot := &snapshots[index]
		snapshot.after, _, snapshot.afterExisted, snapshot.afterErr = readUpdateHookFile(snapshot.path)
	}
}

// restoreUpdateHookFiles returns each hook file to its pre-install bytes, but
// only while it still holds exactly what the candidate's install left: a file
// something else rewrote since — a live chat saving its settings — is never
// clobbered. It is named as residue instead.
func restoreUpdateHookFiles(snapshots []updateHookSnapshot, stderr io.Writer) error {
	var residue error
	for _, snapshot := range snapshots {
		current, _, existed, err := readUpdateHookFile(snapshot.path)
		if err != nil {
			residue = errors.Join(residue, err)
			continue
		}
		if existed == snapshot.beforeExisted && bytes.Equal(current, snapshot.before) {
			continue
		}
		if snapshot.afterErr != nil || existed != snapshot.afterExisted || !bytes.Equal(current, snapshot.after) {
			residue = errors.Join(residue, fmt.Errorf("hook file %s changed after the update's install wrote it; left as is — reconcile it by hand", snapshot.path))
			continue
		}
		if snapshot.beforeExisted {
			err = atomicfile.Write(snapshot.path, snapshot.before, snapshot.beforeMode)
		} else {
			err = os.Remove(snapshot.path)
		}
		if err != nil {
			residue = errors.Join(residue, fmt.Errorf("restore hook file %s: %w", snapshot.path, err))
			continue
		}
		fmt.Fprintf(stderr, "pfm update: restored %s to its pre-update state\n", snapshot.path)
	}
	return residue
}

func readUpdateHookFile(path string) ([]byte, fs.FileMode, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, fmt.Errorf("stat hook file %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, false, fmt.Errorf("hook file %s is not a regular file", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false, fmt.Errorf("read hook file %s: %w", path, err)
	}
	return content, info.Mode().Perm(), true, nil
}

func preferredUpdateSourceRepo(home, repo string) string {
	recorded, err := installer.ReadSourceRepoMarker(home)
	if err != nil {
		return repo
	}
	recordedInfo, recordedErr := os.Stat(recorded)
	repoInfo, repoErr := os.Stat(repo)
	if recordedErr == nil && repoErr == nil && os.SameFile(recordedInfo, repoInfo) {
		return recorded
	}
	return repo
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func updateGitRun(ctx context.Context, repo string, args ...string) error {
	command := exec.CommandContext(ctx, deps.Executable("git"), args...)
	command.Dir = repo
	command.Env = os.Environ()
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(output.String()))
	}
	return nil
}

func updateGitOutput(ctx context.Context, repo string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, deps.Executable("git"), args...)
	command.Dir = repo
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func buildUpdateCandidate(ctx context.Context, repo, version, output string) error {
	moduleRoot := repo
	if _, err := os.Stat(filepath.Join(repo, "pfm", "go.mod")); err == nil {
		moduleRoot = filepath.Join(repo, "pfm")
	}
	// -buildvcs=false: the stage is a git worktree, whose .git is a FILE that
	// cmd/go does not accept as a VCS root, so VCS stamping walks up and dies
	// on any stray .git directory above it ("error obtaining VCS status").
	// The version is stamped through -ldflags, and displayVersion reads VCS
	// info only for an unstamped "dev" build, so nothing is lost. GOFLAGS is
	// cleared below, so the flag must be an argument.
	command := exec.CommandContext(
		ctx,
		deps.Executable("go"),
		"-C", moduleRoot,
		"build", "-trimpath", "-buildvcs=false", "-ldflags", "-X main.version="+version,
		"-o", output,
		"./cmd/pfm",
	)
	command.Env = envWithEmptyGOFLAGS()
	command.Dir = repo
	outputBytes, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build: %w: %s", err, strings.TrimSpace(string(outputBytes)))
	}
	return nil
}

func envWithEmptyGOFLAGS() []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GOFLAGS=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "GOFLAGS=")
}

func fileHash(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}

func copyUpdateFile(source, target string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	return atomicfile.Write(target, raw, info.Mode().Perm())
}

func replaceUpdateFile(source, target string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	return atomicfile.Write(target, raw, info.Mode().Perm())
}

func applyUpdateInstall(ctx context.Context, candidate, repo, sourceRepo string, runtime commandRuntime, skipHarvest bool, stdout, stderr io.Writer) error {
	args := []string{"--yes"}
	if skipHarvest {
		args = append(args, "--skip-harvest")
	}
	return runUpdateCandidateCommand(ctx, candidate, runtime, repo, sourceRepo, stdout, stderr, "install", args...)
}

func runUpdateDoctor(ctx context.Context, candidate string, runtime commandRuntime, skipHarvest bool, stdout, stderr io.Writer) error {
	var args []string
	if skipHarvest {
		args = []string{"--skip-harvest"}
	}
	// Doctor normally inspects the current repository's publication hook as
	// well as host health. An update is validating the newly installed host
	// binary, not whichever source checkout invoked it, so run from a fresh
	// non-repository directory and keep an unwired maintainer checkout from
	// rolling back an otherwise healthy update.
	doctorDirectory, err := os.MkdirTemp("", "pfm-update-doctor-")
	if err != nil {
		return fmt.Errorf("create isolated doctor directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(doctorDirectory); cleanupErr != nil {
			fmt.Fprintf(stderr, "pfm update: cleanup isolated doctor directory %s: %v\n", doctorDirectory, cleanupErr)
		}
	}()
	return runUpdateCandidateCommand(ctx, candidate, runtime, doctorDirectory, "", stdout, stderr, "doctor", args...)
}

func runUpdateCandidateCommand(
	ctx context.Context,
	candidate string,
	runtime commandRuntime,
	workingDirectory string,
	sourceRepo string,
	stdout, stderr io.Writer,
	commandName string,
	commandArgs ...string,
) error {
	args := make([]string, 0, len(commandArgs)+3)
	if runtime.Config.Path != "" {
		args = append(args, "--config", runtime.Config.Path)
	}
	args = append(args, commandName)
	args = append(args, commandArgs...)
	command := exec.CommandContext(ctx, candidate, args...)
	command.Dir = workingDirectory
	if sourceRepo != "" {
		command.Env = updateSourceRepoEnv(sourceRepo)
	}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("target candidate %s: %w", commandName, err)
	}
	return nil
}

func updateSourceRepoEnv(sourceRepo string) []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "PFM_SOURCE_REPO=") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment, "PFM_SOURCE_REPO="+sourceRepo)
}
