# Recording the README fleet GIF

`docs/img/pfm-fleet.gif` is a real `pfm ls` session, recorded inside the dev fence with invented chats — no live chat, project, or account name can reach it.

## Setup (one fence container)

1. A worktree (`.worktrees/demo-gif`, via gitter), then a long-lived fence container on it: `infra/prepare-fence-mounts.sh`, then `docker compose -f infra/docker-compose.yml run -d --build --name pfmdemo pfm-dev sleep infinity` with `PFM_DEV_WORKTREE`, `PFM_DEV_GIT_COMMON`, `PFM_DEV_GIT_DIR_REL` exported as `dev.sh iso` does.
2. Install pfm the normal way inside it: `go build -ldflags "-X main.version=$(cat VERSION)" -o ~/.local/bin/pfm ./cmd/pfm` (the image has no `make`), then `echo '{}' > ~/.claude/settings.json` (what a first Claude Code run leaves) and `pfm install --yes --skip-harvest --skip-themes --skip-engine codex` — the second pass wires the statusline and hooks.
3. A fake `claude` binary (Go, built to `/usr/local/bin/claude`) that keeps the harness contract pfm reads: `argv[0]` is `claude` (pfm matches `/proc/<pid>/cmdline`, so a shell script reads as `bash`); it writes a transcript under `$CLAUDE_CONFIG_DIR/projects/<cwd with / → ->/<sid>.jsonl`, a `custom-title` line from `--name`; it runs the `statusLine` command from `settings.json` with Claude Code's JSON payload every 3s — pfm's statusline writes the session crumb, which is what turns a pane into a `●` live row; and in `-p` print mode it answers `ACK` and exits (pfm's Limits tab sends a one-turn credential refresh).
4. Invented repos (`/work/{api,webapp,ops,docs-site}`, run git from `/tmp` — the container's cwd holds a `.git` file pointing at a host path), then `pfm chat new --name NAME "prompt"` per chat, `pfm chat inject` between chats with `CHAT_SENDER_LABEL` / `CHAT_SENDER_SESSION` set so the ledger can draw the edge, and `pfm chat end` for the `↻` resumable rows.

5. Accounts and limits: `pfm.config.json` carries two Claude seats (`~/.claude`, `~/.cc/2`) and two Codex homes (`~/.codex`, `~/.codex2`, each with a placeholder `auth.json` — pfm's config validation requires one; it is never spent). The Limits tab reads pfm's own usage cache (`/tmp/cc-usage-<uid>/acct-N.json`, `codex-N.json`) and serves a fresh record without a network call, so a seeder writes MOCK numbers there, stamped at recording time. Spawn with `--engine claude` once Codex homes exist.
6. The cosmos warning `N events carry no sender identity`: `pfm chat new` from a bare shell records an unsigned spawn (the spawner is derived from tmux-pane ancestry, not `CHAT_SENDER_*`). Delete exactly those rows from `~/.cc/fleet.db` (`kind='spawn'` with all three sender fields empty); signed `inject` rows stay and draw the edges.

## Recording

VHS 0.12 against Homebrew's ffmpeg 9 prints `Creating …gif` and writes nothing, exit 0. Record frames instead (`Output "frames/"`) and encode by hand:

```bash
ffmpeg -framerate 50 -i frame-text-%05d.png -framerate 50 -i frame-cursor-%05d.png \
  -filter_complex "[0][1]overlay=format=auto,fps=25,pad=w=iw+48:h=ih+48:x=24:y=24:color=0x1e1e2e,scale=1200:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=128:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle" \
  pfm-fleet.gif
```

VHS's window bar, margin and rounded corners are drawn by that same broken ffmpeg step, so the chrome is drawn separately: a Pillow script renders a static frame (violet→indigo→teal gradient backdrop, blurred drop shadow, rounded Catppuccin-Mocha window, title bar with traffic lights and a centered title) and ffmpeg lays the terminal into it — loop the chrome inside the graph (`loop=loop=-1:size=1`) and end on the terminal stream (`overlay=…:shortest=1`); an input-side `-loop 1` never ends and palettegen waits forever. Font: FiraCode Nerd Font Mono 16, line height 1.15.

The tape hides `docker exec -it -w /work/webapp pfmdemo zsh -i`, then records: `pfm ls` → three `Down` → type `pay` → `Backspace 3` → `Tab` ×3 (Stats, Limits, cosmos) → `Escape`. Theme Catppuccin Mocha, 1400×760, font 15. VHS drives headless Chrome, `ttyd`, and the Docker socket, so it runs outside the command sandbox.
