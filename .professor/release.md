# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Engine: harvester — every fetch now runs through ONE gateway. The generic web ladder and the scholarly provider path were two parallel transports, so a provider page behind a JS/bot wall was terminal while the identical wall on the generic ladder was passed. One gateway now owns the SSRF assertion, headers, cookie jar, redirect re-validation, decoding, the byte ceiling, and a single challenge ladder — caller's client, Chrome impersonation, real browser headless, real browser headed. Headless is always attempted before a visible window; the headed rung is last in the gateway and is never spent on a binary download.
- Engine: harvester — a later rung's transport error no longer overwrites an earlier rung's answer. A source that served a challenge page was being reported as a DNS failure, which sent the operator to fix the network instead of the wall.
- Engine: harvester — DNS resolution moves to DNS-over-HTTPS for every dial, with a TTL cache and a LOGGED fallback to the system resolver. A network that answers a source host with its own block address made every rung land on a block page and read as the source refusing the request; resolving over HTTPS removes the class. RFC 6761/6762 special-use names skip it. The SSRF guard is unchanged and still runs on every address returned.
- Engine: harvester — the real-browser rung is pinned to the same DNS-over-HTTPS answers via Chrome's host-resolver rules, closing the gap where Chrome re-resolved independently. Pinning also narrows the DNS-rebinding window the strict fetchable check documents as a residual risk; a private address is never pinned.
