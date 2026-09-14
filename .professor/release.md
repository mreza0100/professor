# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- B: pfm — Harvester sends every fetch through ONE gateway. The generic web ladder and the scholarly provider path were two parallel transports, so a provider page behind a JS/bot wall was terminal while the identical wall on the generic ladder was passed. One gateway now owns the SSRF assertion, headers, cookie jar, redirect re-validation, decoding, the byte ceiling, and a single challenge ladder — caller's client, Chrome impersonation, real browser headless, real browser headed. Headless is always attempted before a visible window; the headed rung is last and is never spent on a binary download. A type-checked source guard fails the build when any other HTTP call shape (a client's Do/Get/Post/Head/PostForm, a RoundTrip, the package-level http helpers) appears outside the gateway, and the web-search health probe `pfm doctor` runs enters it too.
- B: pfm — a later rung's transport error no longer overwrites an earlier rung's answer. A source that served a challenge page was being reported as a DNS failure, which sent the operator to fix the network instead of the wall.
- B: pfm — Harvester resolves every dial over DNS-over-HTTPS, with a TTL cache. A network that answers a source host with its own block address made every rung land on a block page and read as the source refusing the request; resolving over HTTPS removes the class. A transport failure or a server-side DNS error falls back to the system resolver and logs it; NXDOMAIN is an answer, returned as "no such host" without asking the system resolver. RFC 6761/6762 special-use names skip DoH. The SSRF guard still runs on every address returned. (cost): the names of the hosts Harvester fetches are resolved through Cloudflare's resolver (cloudflare-dns.com, reached by its fixed addresses) instead of the system's.
- B: pfm — the real-browser rung is pinned to the same DNS-over-HTTPS answers via Chrome's host-resolver rules, closing the gap where Chrome re-resolved independently. Pinning also narrows the DNS-rebinding window the strict fetchable check documents as a residual risk; a private address is never pinned.
- B: pfm — the real-browser rung's SSRF guard covers WebSockets and service workers. The route guard saw navigation, redirects, subresources and XHR, but a page's `new WebSocket(…)` and its service worker's own fetches never passed through it, so either could reach a private address. WebSocket connections now ask the same fetchable check before connecting (a refusal or a failed check closes the socket), and service workers are blocked for the browser context.
- B: pfm — a Harvester JSON response over its byte ceiling is refused by name. Several scholarly lookups read JSON through a path that silently truncated at the ceiling, so an oversize answer failed as "unexpected end of JSON input"; every JSON read now reports that the response exceeded its byte limit.
- B: pfm — a terminal opened by an app that was itself launched from inside a chat opens the pfm picker again. Such an app (a launcher or window manager a chat relaunched) hands every process it starts the chat's CLAUDECODE / session markers, so each new VS Code terminal looked like a shell inside a chat and auto-open silently stepped aside. The Professor extension's terminal and the fallback PFM profile now drop those markers, and a terminal whose profile still passes them through (a hand-edited profile, an extension not yet reloaded) clears them itself and opens the picker, with one stderr line naming the leak.
- pfm: heal — a wedged/midline projection cursor whose rollout is not canonically ordinalled
  (a repeated, regressed, or skipped ordinal — the openai/codex#38792 duplicate resume-boundary
  variant) now reports NONCANONICAL instead of WEDGED, and one whose rollout could not be read end
  to end reports UNSCANNED; both are new verdicts that are never deleted, report or `--apply`,
  because a rebuild from zero fails on the same record until Codex >= 0.154.0 (PR #42369) projects
  past it. No adopter action.
