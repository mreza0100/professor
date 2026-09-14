# rothgar/awesome-tuis — inclusion research for pfm

Repo: <https://github.com/rothgar/awesome-tuis> (20,583 stars at time of research, MIT-style community list, single `README.md`).

## 1. Mechanics

**File:** `README.md` only (no per-category files). List entries live under `<details open><summary><h2>SectionName</h2></summary> … </details>` blocks. No `CONTRIBUTING.md` exists; contribution rules live in `.github/pull_request_template.md`.

**Entry format** (from neighbours in the Dashboards section):

```
- [oryx](https://github.com/pythops/oryx) A TUI for sniffing network traffic using eBPF
- [otel-tui](https://github.com/ymtdzzz/otel-tui) A terminal OpenTelemetry viewer
- [Planor](https://github.com/mrusme/planor) The Cloud Aviator, dashboard for AWS, Vultr, Heroku, ...
```

`- [Name](url) One-line description.` — name capitalization follows the project's own branding, description is a short, factual, single sentence, no trailing period consistently enforced (mixed in practice).

**Ordering rule within sections:** strict case-insensitive alphabetical by entry name. This was *just* mechanized: PR #888 "Sort list items alphabetically within each section" merged 2026-09-13, and PR #889 "Auto-close PRs and check alphabetical order" (merged same day) added a bot check (`readme_pr_alphabetical.py`) that flags/enforces alphabetical placement on new PRs.

**Inclusion / exclusion criteria** — quoted verbatim from `.github/pull_request_template.md`:
> "Repos need to be at least 6 months old" "Applications should be unique" "Interfaces should be unique. Please don't submit wrappers (e.g. `fzf`)" "Interfaces should be interactive or re-draw output. Output formatters are not considered TUIs"

README top-of-file framing (the wrapper rule, restated): "Commands included in this list should not wrap other interactive commands (e.g. `fzf`), and should be maintained."

**CI / automation** (`.github/workflows/`):

- `link_check.yml` — scheduled weekly `lychee` link checker over `README.md`, opens issues on dead links (not PR-blocking).
- `readme_pr_repo_first_commit.yml` — runs on every PR touching `README.md`. Script `.github/src/readme_pr_repo_first_commit.py`:
  - For each added GitHub URL, fetches the **first commit date** of that repo via the GitHub API (paginates `commits` to the last page).
  - Cutoff = today minus 6 calendar months. If `first_commit_date > cutoff` → status `⛔ too_young`, posts a status table comment, and **the bot itself closes the PR** with a templated comment: *"Closing this PR because every repository added by this change is younger than the 6-month minimum required by this list... please reopen this PR once each of the above repositories has been active for at least 6 months (by first commit)."*
  - Also invokes `readme_pr_alphabetical.py` to check/comment on alphabetical placement.
- No PR template checkbox form beyond the plain markdown text quoted above; no separate `CONTRIBUTING.md`.

## 2. Section and exact insertion point for pfm

Section: **Dashboards** (`<details open><summary><h2>Dashboards</h2></summary>`) — pfm is a fleet-of-chats monitoring/control dashboard, matching entries like `gh-dash`, `damon`, `Planor`, `k9s`-style tools already in this section.

Alphabetical slot: `pfm` (p-f-m) sorts after `otel-tui` (o-t-e) and before `Planor` (p-l-a). Exact current README.md lines (Dashboards section):

```
- [oryx](https://github.com/pythops/oryx) A TUI for sniffing network traffic using eBPF
- [otel-tui](https://github.com/ymtdzzz/otel-tui) A terminal OpenTelemetry viewer
- [Planor](https://github.com/mrusme/planor) The Cloud Aviator, dashboard for AWS, Vultr, Heroku, ...
- [process-compose](https://github.com/F1bonacc1/process-compose) TUI for running apps and processes
```

**Insert immediately between `otel-tui` and `Planor`.**

## 3. What others did — recent add-TUI PRs (checked via `gh pr list`/`gh pr view`, 2026-09-13 activity)

| PR | Title | Outcome | Time to merge | Reason / quote |
| --- | --- | --- | --- | --- |
| #890 | Add tui-do to Productivity | **Closed** (bot) | — | Bot: repo first commit 2026-08-24, "younger than the 6-month minimum" |
| #889 | Auto-close PRs and check alphabetical order | Merged | same-day | Maintainer's own automation PR |
| #888 | Sort list items alphabetically within each section | Merged | same-day | Maintainer's own cleanup PR |
| #887 | Add animpy library to Python libraries list | Merged | same-day | passed age check (no comment shown, script silent-passes) |
| #885 | Add Tomatui | Merged | same-day | first commit 2026-03-07, ✅ passed 6-month check |
| #884 | Add FileView | Merged | same-day | first commit 2026-01-20, ✅ passed |
| #883 | Add FileView and Tomatui | **Closed** | — | duplicate of #884/#885 (both entries ✅ passed age check but PR itself superseded) |
| #882 | Add budget-tracker-tui to Productivity | Merged | same-day | — |
| #880 | Add cmdpeek to Development | **Closed** (self) | — | Author (Esperanza Volkov): *"re-read the contribution requirements... cmdpeek was published very recently, so it doesn't qualify yet"* — self-closed after bot flagged ⛔ (first commit 2026-09-10) |
| #878 | Add local-chat to Messaging | Merged | same-day | — |
| #877 | Add Cloudeval | Merged | same-day | — |
| #875 | Add dbterm | Merged | same-day | — |
| #872 | Add george to Dashboards | **Closed** (maintainer) | — | rothgar: repo first commit 2026-08-29, bot-closed as too young |
| #871 | Add TendKit to Development | **Closed** (maintainer) | — | first commit 2026-08-24, too young |
| #865 | Add baton to Development | **Closed** (maintainer) | — | first commit 2026-06-15, too young |
| #864 | Add swrm to Web | **Closed** (maintainer) | — | first commit 2026-08-29, too young |
| #863 | Add dc-cli | **Closed** (maintainer) | — | first commit 2026-08-13, too young |
| #862 | Add Perkins | **Closed** (maintainer) | — | first commit 2026-04-04, too young |
| #860 | Add pacman-utils | **Closed** (maintainer) | — | first commit 2026-07-18, too young |
| #856 | Add heatsync-tui to Messaging | **Closed** (maintainer) | — | first commit 2026-07-09, too young |
| #855 | Add clipcrate | **Closed** (maintainer) | — | first commit 2026-08-25, too young |

**Patterns observed:**

- The dominant reason for closure by far is the automated **6-month-age** rule — it accounts for the large majority of recent closures, and the bot now self-closes without human involvement in most cases (rothgar occasionally posts manually as well, likely pre-dating full automation).
- Merged PRs in this sample cluster around repos with first-commit dates well past the 6-month mark (e.g. 2026-01-20, 2026-03-07) — no merged PR in this sample had a repo younger than ~6 months.
- No evidence that star count, screenshot, or GIF matters to acceptance — the bot check and PR template make no mention of screenshots/stars/demos; merged entries are one-line README additions with no visual proof required. Low-star projects are routinely merged as long as the age gate passes.
- Duplicate/self-inflicted closures (#883 duplicate, #869/#870 "already added") show maintainer prunes duplicates manually; PR #869 shows a submitter arguing the entry already exists, i.e. no strict duplicate-detection automation, just human/bot review.

## 4. Final entry, branch, commit, PR

**Entry (verbatim, to insert between `otel-tui` and `Planor` in the Dashboards section of `README.md`):**

```
- [pfm](https://github.com/mreza0100/professor) A fleet dashboard for every AI coding chat on your machine (Claude Code, Codex, OpenCode) — fuzzy chat picker, Stats/Limits tabs, a cosmos star-map of chats and messages, cross-chat messaging, and account-preserving reload.
```

**Branch name:** `add-pfm-dashboards`

**Commit message:** `docs: add pfm to Dashboards`

**PR title:** `Add pfm to Dashboards`

**PR body:**

```
Adds pfm, a Go/bubbletea TUI dashboard that lists and controls every AI
coding chat on the machine across accounts (Claude Code, Codex, OpenCode) —
fuzzy fleet picker, Stats/Limits tabs, a cosmos star-map view, cross-chat
messaging, and account-preserving reload. MIT licensed.
```

## 5. Risks

- **Blocking risk — repo age gate:** pfm's repo (`mreza0100/professor`) was created 2026-04-25. Today is 2026-09-14 (~4.7 months old). The automated `readme_pr_repo_first_commit.yml` check computes a first-commit cutoff of "6 calendar months before today" = **2026-03-14**. pfm's first commit (2026-04-25) is **after** that cutoff, so the bot will mark it ⛔ `too_young` and **auto-close the PR** on open, per the exact mechanism observed in PRs #890, #872, #871, #865, #864, #863, #862, #860, #856, #855. This is a near-certain rejection today — not a judgment call, a mechanized gate reading the GitHub API's first-commit date.
  - Earliest safe reopen date: on/after **2026-10-25** (6 months from 2026-04-25).
- Section fit is a judgment call: pfm could arguably also fit "Development" or "Miscellaneous" given its focus on AI coding chats rather than classic system/infra dashboards (compare `gh-dash`, `damon`) — Dashboards was chosen as the closest existing category, but a maintainer could redirect it.
- No CONTRIBUTING.md beyond the PR template; all other criteria (uniqueness, non-wrapper, interactive/redraw) appear satisfied by pfm's description, but are maintainer-judged with no automation covering them, so no certainty here — noted, not verified beyond the text of the entry itself.
- List is high-star/high-traffic (20,583★) with heavy PR volume in a single day (2026-09-13) — many single-line-addition PRs merge quickly once past the bot gate, so mechanics risk (age) dominates over judgment risk.

Sources: `gh repo view`, `gh api repos/rothgar/awesome-tuis/contents/...`, `curl` of raw `README.md`, `.github/pull_request_template.md`, `.github/workflows/*.yml`, `.github/src/readme_pr_repo_first_commit.py`, and `gh pr list` / `gh pr view` (comments) against `rothgar/awesome-tuis`, all fetched read-only, no writes/PRs/issues opened.
