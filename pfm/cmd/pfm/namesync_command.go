package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/gather"
	fleetindex "hostops/pfm/internal/index"
	"hostops/pfm/internal/inject"
	"hostops/pfm/internal/store"
)

// runNameSync converges every live chat's tmux WINDOW name — the fleet's DNS
// record. chat.sh resolves codex chats by it, the terminal tab renders it, and
// a person picking a pane reads it.
//
// A codex window follows its thread's indexed name; a claude window follows
// the 🔖 label its own statusline renders. Both halves are computed and
// applied by the same gather pass the picker runs, so there is exactly ONE
// writer of a window name however this command is reached — a systemd path
// unit on a codex rename, a timer, or a picker refresh.
func runNameSync(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	flags := newFlagSet("name-sync", "usage: pfm name-sync [--dry-run]", stderr)
	dryRun := flags.Bool("dry-run", false, "report the renames without applying them")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}
	defer database.Close()
	ctx := context.Background()

	// A delta index first: a codex rename lands in session_index.jsonl or the
	// thread store, and the name a window converges on is read from the index.
	// Without this pass the sync would converge yesterday's names.
	indexer, err := fleetindex.NewWithRoots(database, runtime.Paths, runtime.Paths.Roots)
	if err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}
	if _, err := indexer.Run(ctx, fleetindex.Options{}); err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}

	environment, err := resolveScanEnvironment(scanRequest{Runtime: &runtime})
	if err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}
	data, err := loadFleetData(ctx, database)
	if err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}
	// ReadOnly is what makes --dry-run a dry run: the gather pass applies the
	// renames it plans, and only a read-only pass plans without applying.
	live, err := gatherFleet(
		ctx,
		database,
		environment.paths,
		environment.config,
		data,
		*dryRun,
		printWarn(stderr),
		stderr,
	)
	if err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}
	if !*dryRun {
		reconcileCodexPanes(ctx, database, live, runtime, printWarn(stderr))
	}
	verb := "renamed"
	if *dryRun {
		verb = "would rename"
	}
	for _, rename := range live.Renames {
		fmt.Fprintf(
			stdout,
			"%s %s %s: %s -> %s\n",
			verb,
			rename.Socket,
			rename.WindowID,
			rename.CurrentName,
			rename.TargetName,
		)
	}
	if *dryRun {
		// A dry run applied nothing, so it has nothing to verify. It reports
		// the PLAN, and says so — a plan counted as an outcome is exactly the
		// lie this command used to tell.
		fmt.Fprintf(stdout, "windows planned: %d\n", len(live.Renames))
		return 0
	}
	titlesTmux := gather.CommandTmux{TmuxTmpDir: filepath.Dir(environment.paths.TmuxDir)}
	_, titlesUnverified := convergeTmuxTitles(
		ctx,
		titlesTmux,
		liveSockets(live.Panes),
		environment.config.Tmux.Titles,
		stdout,
		stderr,
	)
	if titlesUnverified != 0 {
		fmt.Fprintf(stdout, "tmux titles unverified: %d\n", titlesUnverified)
	}
	converged, unverified := verifyRenames(ctx, runtime, live.Renames, stderr)
	fmt.Fprintf(stdout, "windows converged: %d\n", converged)
	if unverified != 0 {
		fmt.Fprintf(stdout, "windows unverified: %d\n", unverified)
	}
	if titlesUnverified != 0 || unverified != 0 {
		return 1
	}
	return 0
}

// verifyRenames reads every renamed window's name BACK off its server and
// counts only a match as converged.
//
// An attempt is not an outcome. `windows converged: N` used to count the
// renames this pass planned, so a window whose name a second writer took back
// — or one whose server died between the plan and the rename — was reported as
// converged while the fleet still could not address it by that name. Each
// unverified window is named with the value actually read, and the command
// exits non-zero so a scheduler run that achieved nothing is not silent.
func verifyRenames(
	ctx context.Context,
	runtime commandRuntime,
	renames []gather.WindowRename,
	stderr io.Writer,
) (converged, unverified int) {
	reader := inject.CommandTmux{}
	for _, rename := range renames {
		socketPath := filepath.Join(runtime.Paths.TmuxDir, rename.Socket)
		actual, err := reader.WindowName(ctx, socketPath, rename.WindowID)
		if err != nil {
			unverified++
			fmt.Fprintf(
				stderr,
				"pfm name-sync: window %s %s: wanted %q, could not be read back after rename: %v\n",
				rename.Socket, rename.WindowID, rename.TargetName, err,
			)
			continue
		}
		if actual != rename.TargetName {
			unverified++
			fmt.Fprintf(
				stderr,
				"pfm name-sync: window %s %s: wanted %q, reads %q after rename\n",
				rename.Socket, rename.WindowID, rename.TargetName, actual,
			)
			continue
		}
		converged++
	}
	return converged, unverified
}

// liveSockets returns the DISTINCT sockets backing panes, sorted. name-sync
// already probed these servers to plan window renames; titles convergence
// reuses that same enumeration rather than probing the tmux directory a
// second time.
func liveSockets(panes []gather.Pane) []string {
	seen := make(map[string]bool, len(panes))
	sockets := make([]string, 0, len(panes))
	for _, pane := range panes {
		if seen[pane.Socket] {
			continue
		}
		seen[pane.Socket] = true
		sockets = append(sockets, pane.Socket)
	}
	sort.Strings(sockets)
	return sockets
}

// convergeTmuxTitles is the ONE place an EXISTING live server's tmux.titles
// state is brought onto the machine's intended policy. The three creation
// doors (spawn.CommandTmux.NewSession, action.CommandTmux.CreateCodexServer,
// the pfm.zsh shim) apply the policy once, at birth — nothing converged a
// server that predates a policy change, or one a scheduler outage left
// behind, until this pass. It runs on every scheduled name-sync alongside the
// window-name convergence it already performs.
//
// A HOST-owned policy (Enabled == false) touches nothing on any server: that
// policy exists precisely so a host that owns its own OSC title before tmux
// starts keeps it, and a server already `set-titles off` for that reason is
// not drift.
//
// A PFM-owned policy converges any server that diverges — `set-titles` not
// `on`, or the string not config.TmuxTitlesString — by applying
// config.TmuxTitles.Options() to it, the exact argv the creation doors use.
// Every pfm-owned server, converged or already correct, then gets the same
// flip-and-restore nudge tmux-title-renudge performs, so a terminal caching a
// stale title (a VS Code tab reviving a persistent pane) repaints even when
// the server's OPTIONS never changed.
//
// Convergence is reported per server — a named transition, never silence —
// plus one summary count; a read or apply failure on one socket is named on
// stderr and does not stop the pass from converging the rest.
func convergeTmuxTitles(
	ctx context.Context,
	tmux gather.CommandTmux,
	sockets []string,
	titles pfmconfig.TmuxTitles,
	stdout, stderr io.Writer,
) (converged, unverified int) {
	if !titles.Enabled {
		return 0, 0
	}
	for _, socket := range sockets {
		actualTitles, titlesErr := tmux.ShowGlobalOption(ctx, socket, "set-titles")
		if titlesErr != nil {
			fmt.Fprintf(stderr, "pfm name-sync: tmux titles %s: could not read set-titles: %v\n", socket, titlesErr)
			unverified++
		}
		actualString, stringErr := tmux.ShowGlobalOption(ctx, socket, "set-titles-string")
		if stringErr != nil {
			fmt.Fprintf(stderr, "pfm name-sync: tmux titles %s: could not read set-titles-string: %v\n", socket, stringErr)
			unverified++
		}
		if titlesErr != nil || stringErr != nil {
			continue
		}
		titlesOff := actualTitles != "on"
		stringWrong := actualString != pfmconfig.TmuxTitlesString
		if titlesOff || stringWrong {
			if err := tmux.ApplyGlobalOptions(ctx, socket, titles.Options()); err != nil {
				fmt.Fprintf(stderr, "pfm name-sync: tmux titles %s: convergence failed: %v\n", socket, err)
				unverified++
				continue
			}
			verifiedTitles, verifyTitlesErr := tmux.ShowGlobalOption(ctx, socket, "set-titles")
			if verifyTitlesErr != nil {
				fmt.Fprintf(stderr, "pfm name-sync: tmux titles %s: could not verify set-titles after apply: %v\n", socket, verifyTitlesErr)
				unverified++
				continue
			}
			verifiedString, verifyStringErr := tmux.ShowGlobalOption(ctx, socket, "set-titles-string")
			if verifyStringErr != nil {
				fmt.Fprintf(stderr, "pfm name-sync: tmux titles %s: could not verify set-titles-string after apply: %v\n", socket, verifyStringErr)
				unverified++
				continue
			}
			if verifiedTitles != "on" {
				fmt.Fprintf(stderr, "pfm name-sync: tmux titles %s: set-titles read back %q after apply, wanted %q\n", socket, verifiedTitles, "on")
				unverified++
			}
			if verifiedString != pfmconfig.TmuxTitlesString {
				fmt.Fprintf(stderr, "pfm name-sync: tmux titles %s: set-titles-string read back %q after apply, wanted %q\n", socket, verifiedString, pfmconfig.TmuxTitlesString)
				unverified++
			}
			if verifiedTitles != "on" || verifiedString != pfmconfig.TmuxTitlesString {
				continue
			}
			switch {
			case titlesOff && stringWrong:
				fmt.Fprintf(
					stdout,
					"tmux titles %s converged: set-titles %s -> on, set-titles-string %q -> %q\n",
					socket, actualTitles, actualString, pfmconfig.TmuxTitlesString,
				)
			case titlesOff:
				fmt.Fprintf(stdout, "tmux titles %s converged: set-titles %s -> on\n", socket, actualTitles)
			default:
				fmt.Fprintf(
					stdout,
					"tmux titles %s converged: set-titles-string %q -> %q\n",
					socket, actualString, pfmconfig.TmuxTitlesString,
				)
			}
			converged++
		}
		// The nudge runs on every pfm-owned server, converged or already
		// correct, so a terminal caching a stale title still repaints.
		if err := tmux.NudgeTitlesString(ctx, socket, pfmconfig.TmuxTitlesString); err != nil {
			fmt.Fprintf(stderr, "pfm name-sync: tmux titles %s: %v\n", socket, err)
			unverified++
		}
	}
	fmt.Fprintf(stdout, "tmux titles converged: %d\n", converged)
	return converged, unverified
}
