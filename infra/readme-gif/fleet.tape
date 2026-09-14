# VHS tape for the README hero GIF — record.sh runs it from tmp/readme-gif/.
# Output is raw frame layers: VHS 0.12 against Homebrew ffmpeg 9 announces a
# GIF and writes nothing, and its window chrome is drawn by that same failing
# step, so record.sh composites and encodes the frames itself.
Output "frames/"

Set Shell "bash"
Set FontFamily "FiraCode Nerd Font Mono"
Set FontSize 16
Set LineHeight 1.15
Set Width 1440
Set Height 840
Set Padding 14
Set Theme "Catppuccin Mocha"
Set TypingSpeed 95ms

# Seeding right before the take keeps the Limits cards "confirmed seconds ago".
Hide
Type "docker exec -it -w /work/webapp -e TERM=xterm-256color -e COLORTERM=truecolor pfm-readme-gif zsh -i"
Enter
Sleep 3s
Type "python3 /worktree/infra/readme-gif/seed-limits.py >/dev/null; clear; cd /work/webapp"
Enter
Sleep 1s
Show

Sleep 1.6s
Type "pfm ls"
Sleep 500ms
Enter
Sleep 3.2s
Down
Sleep 450ms
Down
Sleep 450ms
Down
Sleep 800ms
Type "pay"
Sleep 2s
Backspace 3
Sleep 1s
# Stats, then Limits (5s), then the cosmos sky (6.5s).
Tab
Sleep 1.4s
Tab
Sleep 5s
Tab
Sleep 6.5s
Escape
Sleep 2.2s
