---
name: quality:description
description: MANDATORY — load before writing or certifying any `description:` (command, skill, agent, MCP tool or server); grammar, per-kind char caps, cut order, family / USER-ONLY / MCP self-containment law, Approval gate (APPROVED/REJECTED). General prompt law → /quality:prompt.
argument-hint: [path…]
---

# Description Law

Every `description:` is injected into every session and every sub-agent spawn, whether or not its body ever loads — a whole registry of them, re-paid per spawn, for routing alone. `/quality:prompt` governs every prompt line; this file adds only what a description needs beyond it: what to say, in what order, how much, and how to certify it. With `[path…]` the command runs § Approval over each path (a `.md` frontmatter, or a Go file's `Description:` / `Instructions:` strings) and emits one verdict per entry.

## Grammar — four components, one order

```
[TOKEN] {function} — {when}. [Returns {shape}.] [Not for {x} → {home}.]
```

- TOKEN (conditional): the invocation class, first word, one canonical spelling — `MANDATORY` (must load/route at the named step), `USER-ONLY` (a human types it; § USER-ONLY), `{CALLER}-ONLY` (invoked only by the named entity, e.g. `ORCHESTRATOR-ONLY`). No token = the model routes freely.
- function (required unless the name already says it): ≤ 5 words, verb first, ends at the first ` — `. `Lint/format markdown`, `Writes ONE zero-gap wave spec`. Details, modes and artifacts belong to `when`, never here. Omitted → `TOKEN — {when}` (`USER-ONLY — the user types /reload; …`).
- when (required): the call-to-action — the asks it answers (≤ 3 trigger phrasings), every subcommand / mode / flag the body handles in its typed form (an unnamed entry point is unroutable), and for a family member its neighbours (§ Family).
- Returns (agents and MCP tools; conditional elsewhere): the artifact kind only — a path, a map, a verdict, ONE line.
- Not for (conditional): a redirect, never a bare ban — `Not for static context size → /context-meter`. Include it only when a real misroute exists; the redirect target is what makes it positive.

The description is the WHEN and WHAT; the body is the HOW. A rule the body enforces, a procedure step, or a quality bar ("zero-gap", "quote-pinned", "closed-world") leaves the description unless the caller must choose by it.

## Budget — caps that are measured, never estimated

- command / skill / agent (body-bearing): ≤ 280 chars; ≤ 400 when every char past 280 is an entry point, a family neighbour, or the Returns clause.
- MCP tool (no body): ≤ 600 chars, one of them an inline example call; argument docs live in the input schema, never narrated.
- MCP server `Instructions`: ≤ 900 chars — the routing map (ask-phrase → tool) once, the audience boundary once, confusable siblings contrasted.
- harness ceiling: a skill's `description` + `when_to_use` ≤ 1,536 chars combined, 1,024 alone — the harness refuses above it; never the target.

Why 280: the four components at their densities — function ≈ 30 chars, when ≈ 130 (3 triggers + 2–3 entry points), token/Returns ≈ 60, redirect ≈ 60 — sum to 280; anything past that is either a fifth entry point (the 400 tier) or scaffolding. A registry rewritten to the cap roughly halves its per-spawn cost. Measure with this — it stops at the closing `---` as well as at the next key, and never reads the description's own line as a terminator (both were real miscounts, one reporting a 664-char description as 16,288):

```bash
awk 'f{ if (/^---[[:space:]]*$/ || /^[A-Za-z_-]+:/) exit; gsub(/^[[:space:]]+/,""); s=s " " $0; next }
     /^description:/{ f=1; sub(/^description:[[:space:]]*[>|]?-?[[:space:]]*/,""); s=$0 }
     END{ gsub(/^[[:space:]]+|[[:space:]]+$/,"",s); print length(s) }' FILE
```

Cut order when over cap (first cut first): 1 the mechanism — library, engine, internal pipeline stage the caller never selects (`with rumdl`, `via midrun.js`, `Sonnet walkers`, `zero-token rule engine`); 2 the name echo (§ Naming); 3 rationale and body-enforced rules; 4 Returns detail beyond the artifact kind; 5 trigger phrasings beyond three; 6 the `Not for` clause when a harness mechanism already enforces it (`disable-model-invocation`, a `tools:` allowlist). Never cut: the function, the TOKEN, a family neighbour, the MCP example call.

## Naming

The name is the first routing word and is read beside every description: `family:verb-noun`, two words whose pairing is self-explanatory (`quality:prompt`, `wave:refine`, `h:gh`). The description never respends the name's words: `wave:refine` opening `Wave refinement —` and `wave:builder` opening `The wave builder —` pay for what the reader already has. When the name states the function fully (`git`, `reload`, `quality:prompt`), the function clause is omitted and the description opens with the TOKEN or `when`.

## Family

A `family:*` set is one pipeline, and its order must be legible from the descriptions alone, without opening a body. The entry member carries the chain once, in the `when` clause, as arrows: `Chain head: refine → /wave:orchestrator → /wave:builder → /wave:walker`. Every other member names only its predecessor and successor (`after /wave:refine; hands each wave to /wave:builder`) and, when it is not user-invoked, its `{CALLER}-ONLY` token. Cross-family hand-offs use the same arrow (`Not for static context size → /context-meter`). The chain lives in exactly one member; a second copy is duplication that rots on the next insert.

## USER-ONLY — human-triggered entities

An entity the user alone triggers (`/reload`, `/handoff`, `/deep-rr`, `/wave:ccc`) declares it twice, because a prompt clause is advisory and the harness flag is not:

- frontmatter `disable-model-invocation: true` — the harness then omits the description from the model's registry; a self-invocation is unreachable, not merely forbidden.
- the description opens `USER-ONLY — the user types /{name}; never run it unprompted.` — for the human's menu, and for every copy of the description a runtime without the flag reads.

A `USER-ONLY` prefix without the flag is the weaker half alone; the flag without the prefix leaves the mirrors blind. § Approval check 6 rejects either — with one exception it also records: an entity the user may ASK for in prose ("reload this chat", "research X for me") keeps the token and deliberately omits the flag, because the flag makes the route unreachable for a request the user actually made. State that in the description's own clause (`the user types /reload, or asks for it`), and note that a skill has no such flag at all: there the token is the only mechanism, and a caller-side `tools:` allowlist is the real one.

## MCP tools — self-contained, because there is no body

An MCP tool's description is all a caller will ever read, next to a JSON input schema. Two levels, each said once:

- Server `Instructions`: the family map — each tool's ask-phrase (`"send / tell / message chat X" is chat_inject`), the confusable pairs contrasted (`archive` browses compressed archives, NOT a webpage), the audience boundary (`cross-chat between independent chats, never parent/child`), and the cache/ordering habit that applies to every tool (`prefer searchCache before re-fetching`).
- Per-tool `Description`, in the § Grammar order, plus three parts a body would otherwise carry:
  1. one inline example call with the schema's real field names — `fetch{sources:["https://…","doi:10.…"]}`, `chat_inject{target:"pane-name", message:"…"}`; a tool that lacks its example is the one agents misuse.
  2. the hand-off to a sibling when its output feeds one (`findWorks` returns a handle → `fetch`), and the scope boundary when the tool is misusable — WHO may call it and for WHAT: `chat_*` verbs are for one chat addressing another running chat; a sub-agent returns its result and its parent reads it, so a sub-agent never holds a reason to call `chat_inject`, `chat_self_compact`, or `chat_goal`.
  3. what its failure and its empty result look like, distinguished: `empty list = nothing cached; error = the lookup failed` — never one shape for both.

Argument narration (`set size_only to…`, `refresh re-downloads…`) duplicates the schema's own field descriptions — cut it from the description, sharpen it in the schema. And the prompt clause is only half the parent/child fix: a sub-agent's `tools:` allowlist never carries `mcp__chat__*`; an agent defined with `tools: *` is where the misuse enters.

## WRONG → RIGHT — from a live registry

Chars measured on the live text; the RIGHT column obeys § Grammar and § Budget.

### A markdown command — 361 → 207 chars

WRONG: `Lint and format markdown with rumdl — `check [path]` reports, `fmt [path]` rewrites in place, `prompt-safe <file>` formats a template carrying machine-read markers and verifies it still renders. Use when markdown is inconsistently wrapped, before committing a doc or spec, or when adding a formatter to a project. Route markdown lint/format/style requests here.`

RIGHT: `Lint/format markdown — `check [path]` reports, `fmt [path]` rewrites, `prompt-safe <file>` keeps machine-read markers intact. Route every markdown lint/format ask here; `fmt` before committing a doc or spec.`

Cut: the mechanism (`with rumdl`), the rationale ("inconsistently wrapped"), the third trigger.

### `/wave:refine` — 479 → 295 chars, and the pipeline becomes legible

WRONG opens `Wave refinement — walks the code and writes ONE zero-gap, feature-scoped wave spec (tasks inside-out) …` and names the scheduler agent, never the command that consumes the spec: a reader cannot place it in `/wave:*`.

RIGHT: `Writes ONE zero-gap wave spec to docs/dev/trains/queue/{date}-{slug}.md, asking only what the code cannot answer. Chain head: refine → /wave:orchestrator → /wave:builder → /wave:walker. `poc <goal>` refines AND builds under .professor/RND/POC/{name}/. Triggers "refine", "refine this/tasks/poc".`

Over 280 by the chain and the `poc` entry point — the 400 tier, spent as the tier allows.

### A code-review agent — 831 → 306 chars

WRONG carries the procedure (`every lane read as whole bodies producer→surface, diff angles for removed behavior and dishonest failure, the project's own tests run with the diff's flags flipped`), the seat mechanics (`Sonnet only, batches its seats (≤7 agents)`), and the ledger format (`F{n} · open/resolved/waived`) — body content.

RIGHT: `Reviews a diff range or code lane, every hunk ledgered, tests run — returns ONE line: the report path. Delegate for "review this branch/range/merge", "is this correct", or after a tracer map; the default where /code-review would be used. Wave dir in → REVIEW.md ledger gitter reads before merge. Read-only.`

### `chat_self_compact` MCP tool — 1,296 → 485 chars

WRONG states each rule twice — once as the rule, once as the incident that produced it (`a caller that keeps working after calling this makes its own turn indistinguishable from the compaction and the steer lands beside…`) — and has no example call.

RIGHT: `Compacts THIS chat in place after its turn settles; the session (crons, sub-agents, pane) survives — the only route for "compact yourself" / "self-compact"; never a typed /compact or a reload. Call: chat_self_compact{focus:"one line", then:"ONE steer string, not starting with /compact"}. Only focus+then cross the boundary: write durable state to disk FIRST. After it returns, END THE TURN — further work lands the steer beside the compaction. Main chat only; a sub-agent has no pane.`

Every rule survives; the narration and the missing audience line are the difference.

### `fetch` MCP tool — 993 → 414 chars

WRONG narrates schema fields the schema already documents (`Set size_only to return sizes and path without an inline body, or refresh to retrieve a fresh copy`), lists supported formats (a server-level routing fact), and carries `Acquisition details remain internal` — routing nothing.

RIGHT: `Retrieves 1–50 documents as Markdown, input order kept. Call: fetch{sources:["https://…","doi:10.…","harvest:…"]} — a URL/path, DOI, ISBN, PMID/PMCID, or a handle from findWorks/searchCache; a title → findWorks first. Each item returns content (may be truncated), size, cache status, and the path to the COMPLETE artifact — read the path for the rest. A failing item carries its own error; the others still return.`

## Approval — certify a description

Run over every entry, at write-time and on demand. An entry is APPROVED only when ALL hold; otherwise REJECTED with the failing checks named, fixed, and re-checked. Measure check 2 with the awk line in § Budget — a number, never an estimate.

- 0 Present: the `description:` (or a tool's `Description:`) is absent or empty — an unroutable entry.
- 1 Grammar: a TOKEN is not the first word; the function clause runs past 5 words or past the first ` — `; a component sits out of § Grammar order.
- 2 Budget: a body-bearing entry exceeds 280 chars without the 400-tier justification, or 400 at all; an MCP tool exceeds 600; server instructions exceed 900.
- 3 Name echo: the opening clause restates the name's words.
- 4 Mechanism: a library, engine, model tier, or internal pipeline stage the caller never selects is named.
- 5 Entry point: a subcommand, mode, flag, or alias the body handles is missing from `when`.
- 6 Token: an obligation the body or CLAUDE.md states lacks its TOKEN; `USER-ONLY` appears without `disable-model-invocation: true` and without § USER-ONLY's spoken-request clause, or the flag appears without the prefix.
- 7 Family: a member lacks predecessor/successor; the chain is missing from the entry member, or present in a second one.
- 8 Redirect: a `Not for` / `never` clause names no home, or duplicates an enforced harness mechanism.
- 9 MCP: a tool lacks its example call with real field names, a confusable sibling is not contrasted, empty and error results share one shape, or the description narrates schema fields.
- 10 Body echo: the body's opening restates the description.

Emit per entry `APPROVED: {path}` or `REJECTED: {path} — checks {n,…}`; a file that could not be read or parsed emits `UNREAD: {path} — {error}`, never a verdict. A family or an MCP server is approved only when every member is.
