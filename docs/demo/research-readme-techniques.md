# RR — How the best-presented AI/dev-tools/CLI GitHub READMEs are structured, and what to copy

Question: find how the biggest, best-presented GitHub projects in the AI/dev-tools/CLI space structure their README, and return a ranked catalogue of concrete components and techniques we can copy for our own README.

## Answer

Top READMEs converge on a small set of GitHub-native HTML/markdown mechanisms (badges, centered hero + tagline, `<picture>` dark/light logos, `<details>` collapsibles, VHS-generated GIF demos) layered over a predictable section order (hero → quickstart → features/why → install → docs/contributing); the highest-leverage move for a CLI/dev-tool README is a `<picture>`-swapped hero image plus a VHS-produced terminal GIF within the first screen, since GitHub strips all CSS/JS and these are the only "animated/theme-aware" primitives that actually render.

Sampled repos (18, all fetched via raw README): anthropics/claude-code, openai/codex, sst/opencode, Aider-AI/aider, block/goose, charmbracelet/bubbletea, charmbracelet/gum, jesseduffield/lazygit, astral-sh/uv, astral-sh/ruff, BurntSushi/ripgrep, junegunn/fzf, ollama/ollama, zed-industries/zed, langchain-ai/langchain, crewAIInc/crewAI, cli/cli, tldr-pages/tldr. All fetches succeeded — no failures to report. matiassingers/awesome-readme was fetched for its catalogue of technique categories.

## Ranked catalogue of techniques

| # | Technique | Repos using it (of 18 sampled) | GitHub-markdown mechanism | Why it works |
|---|---|---|---|---|
| 1 | Shields.io badge row (build, version, license, chat) | All 18 | `[![label](https://img.shields.io/...)](link)`, plain `<img>`/`<a>` — no CSS needed | Instant credibility/health signal; zero rendering risk since it's just linked PNGs |
| 2 | Centered hero via `<p align="center">` / `<div align="center">` | claude-code, codex, opencode, aider, goose, bubbletea, gum, uv, ruff, ollama, crewAI, tldr, langchain (13/18) | `<p align="center"><img ...></p>` — one of the few alignment hacks GitHub honors | Only way to center anything since GitHub strips `text-align` CSS classes elsewhere |
| 3 | One-line tagline immediately under title/logo | All 18 | Plain markdown text, no special markup | Answers "what is this" in <2 seconds, before any scroll |
| 4 | GitHub alert callouts `> [!NOTE]` / `[!TIP]` / `[!IMPORTANT]` | claude-code, opencode, bubbletea, gum, cli/cli, tldr (6/18) | Native GFM alert blockquote syntax | Visually distinct colored box, renders natively, no image/HTML needed |
| 5 | `<picture>` element for dark/light-mode logo or benchmark chart | opencode, uv, ruff, langchain, tldr, VHS itself (6/18 + VHS) | `<picture><source media="(prefers-color-scheme: dark)" srcset=...><img ...></picture>` — GitHub explicitly whitelists `<picture>`/`<source>` | Only theme-adaptive mechanism GitHub supports; looks broken in one theme without it |
| 6 | Animated terminal demo (GIF, often VHS-generated) | claude-code, aider, bubbletea, gum, lazygit (5/18) | `<img src="demo.gif" width="600">` or plain `![demo](demo.gif)` | Shows the product working in the first screen — highest comprehension lift of any single element |
| 7 | `<details><summary>` collapsible sections | codex, gum, fzf (3/18) | Native HTML, works with zero JS | Hides OS-specific install variants / long examples without bloating first screen |
| 8 | Benchmark/comparison table (markdown table) | ripgrep, fzf | Plain GFM pipe tables | Concrete, skimmable proof of the core value prop (speed, feature parity) |
| 9 | "Why X?" / "Why shouldn't I use X?" section | ripgrep, langchain, crewAI, aider (values-prop framing) | Plain markdown heading + prose | Pre-empts the reader's evaluation question directly instead of making them infer it from features |
| 10 | Feature grid with emoji bullets | ruff (⚡️🐍🛠️🤝), aider (icon+text repeats) | Plain markdown list with emoji, sometimes small inline `<img>` icons | Scannable value props without needing real graphic design |
| 11 | Contributors/sponsors avatar strip | lazygit (sponsor avatars in HTML comment-marked block), tldr (contributor badge) | `<img>` grid or shields.io contributor-count badge; contrib.rocks-style generators not found directly in sample | Social proof, but was the least common technique in this sample |
| 12 | Star-history / ranking badge (Trendshift) | crewAI, goose | `<a href="https://trendshift.io/..."><img src="https://trendshift.io/api/badge/..."></a>` | External image-service badge — same low-risk mechanism as shields.io, signals momentum |
| 13 | Multi-language README links row | opencode (21 languages) | Plain markdown link row (`[简体中文](README.zh.md)`) | Broadens audience; trivial to add once translations exist |
| 14 | Telemetry/data-usage transparency section | claude-code, crewAI | Plain markdown section, sometimes with env-var opt-out snippet | Builds trust for AI/agent tools specifically — a category-specific pattern, not generic README advice |

Not found in the sampled repos: contrib.rocks-style auto-generated contributor image walls, star-history.com chart embeds, and animated SVG (svg-term/termtosvg) demos — none of the 18 READMEs embedded a live star-history chart or an SVG terminal recording directly, only GIFs and `<picture>`-wrapped raster images.

## First-screen anatomy (top 5: claude-code, codex, opencode, aider, uv/ruff pattern)

Above the fold in the strongest READMEs is consistently: (1) centered logo/wordmark, sometimes `<picture>`-swapped for dark/light; (2) one-line tagline stating what the tool is and who it's for; (3) a badge row (version, build, license, community); (4) either a quickstart code block (codex, claude-code) or an animated GIF/benchmark chart (aider, uv, ruff) — never both stacked, one or the other keeps the fold light. crewAI and goose add a centered navigation link bar (Homepage/Docs/Discord) as a fifth element. Sources: raw READMEs of [anthropics/claude-code](https://raw.githubusercontent.com/anthropics/claude-code/main/README.md), [openai/codex](https://raw.githubusercontent.com/openai/codex/main/README.md), [sst/opencode](https://raw.githubusercontent.com/sst/opencode/master/README.md), [Aider-AI/aider](https://raw.githubusercontent.com/Aider-AI/aider/main/README.md), [astral-sh/uv](https://raw.githubusercontent.com/astral-sh/uv/main/README.md).

## Terminal-demo tooling and GitHub embedding

- **VHS (charmbracelet)**: write a declarative `.tape` script (window size, theme, typing speed, commands) and render to GIF/MP4/WebM. Its own README embeds the demo with a `<picture>` wrapper around the GIF (dark/light `<source>` tags currently pointing to the same file, `<img>` fallback with fixed `width` and alt text) — a pattern also copied by charmbracelet/gum. Renders natively on GitHub since `<picture>`/`<source>`/`<img>` are on GitHub's HTML allowlist. [charmbracelet/vhs README](https://raw.githubusercontent.com/charmbracelet/vhs/main/README.md)
- **asciinema**: produces `.cast` files whose JS player does not execute on GitHub (GitHub strips `<script>`). The standard workaround is converting the cast to a static asset: `svg-term-cli` renders it to an animated SVG (`svg-term --cast=ID --out demo.svg`), or `termtosvg` records directly to SVG; both are then embedded as plain `![demo](demo.svg)`. A GIF conversion path also exists as a fallback when SVG isn't wanted. [svg-term-cli](https://github.com/marionebl/svg-term-cli), [termtosvg](https://github.com/nbedos/termtosvg) (archived June 2020).
- **Practical takeaway for GitHub**: VHS → GIF is the path with the most precedent in this sample (5 of 18 repos); asciinema → SVG is documented but was not observed live in any of the 18 sampled READMEs, so treat it as available-but-unproven-at-this-tier rather than a default choice.

## Open questions

- Whether asciinema itself now offers a native GIF/SVG export (vs. only third-party converters) is unresolved — the CLI's own export docs weren't fetched directly; flagged as a gap by the awesome-readme digger, not verified here.
- GitHub's exact HTML sanitizer allowlist (beyond the tags observed working: `p`, `div`, `picture`, `source`, `img`, `details`, `summary`, `kbd`, `a`) was not independently confirmed against GitHub's own docs — inferred from what renders in the sampled READMEs, not from a fetched sanitizer spec.
- Contrib.rocks-style contributor walls and star-history.com charts were not found in any of the 18 sampled repos; this is reported as absence-in-sample, not evidence they're rare industry-wide, since the sample skews CLI/AI-agent tools rather than large community-driven libraries where those widgets are more common.

Sources:
- [anthropics/claude-code README](https://raw.githubusercontent.com/anthropics/claude-code/main/README.md)
- [openai/codex README](https://raw.githubusercontent.com/openai/codex/main/README.md)
- [sst/opencode README](https://raw.githubusercontent.com/sst/opencode/master/README.md)
- [Aider-AI/aider README](https://raw.githubusercontent.com/Aider-AI/aider/main/README.md)
- [block/goose README](https://raw.githubusercontent.com/block/goose/main/README.md)
- [charmbracelet/bubbletea README](https://raw.githubusercontent.com/charmbracelet/bubbletea/main/README.md)
- [charmbracelet/gum README](https://raw.githubusercontent.com/charmbracelet/gum/main/README.md)
- [jesseduffield/lazygit README](https://raw.githubusercontent.com/jesseduffield/lazygit/master/README.md)
- [astral-sh/uv README](https://raw.githubusercontent.com/astral-sh/uv/main/README.md)
- [astral-sh/ruff README](https://raw.githubusercontent.com/astral-sh/ruff/main/README.md)
- [BurntSushi/ripgrep README](https://raw.githubusercontent.com/BurntSushi/ripgrep/master/README.md)
- [junegunn/fzf README](https://raw.githubusercontent.com/junegunn/fzf/master/README.md)
- [ollama/ollama README](https://raw.githubusercontent.com/ollama/ollama/main/README.md)
- [zed-industries/zed README](https://raw.githubusercontent.com/zed-industries/zed/main/README.md)
- [langchain-ai/langchain README](https://raw.githubusercontent.com/langchain-ai/langchain/master/README.md)
- [crewAIInc/crewAI README](https://raw.githubusercontent.com/crewAIInc/crewAI/main/README.md)
- [cli/cli README](https://raw.githubusercontent.com/cli/cli/trunk/README.md)
- [tldr-pages/tldr README](https://raw.githubusercontent.com/tldr-pages/tldr/main/README.md)
- [matiassingers/awesome-readme](https://github.com/matiassingers/awesome-readme)
- [charmbracelet/vhs README](https://raw.githubusercontent.com/charmbracelet/vhs/main/README.md)
- [svg-term-cli](https://github.com/marionebl/svg-term-cli)
- [termtosvg](https://github.com/nbedos/termtosvg)
