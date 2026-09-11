# RR — Which Unicode code points does VS Code's xterm.js WebGL renderer custom-draw, and what rendering defects are reported against that path?

Question: determine precisely which Unicode characters VS Code's integrated terminal (xterm.js) WebGL renderer draws itself as "custom glyphs" instead of rasterizing from the font, and which known rendering defects (stale cells, blank cells, ghost text from previous frames, wrong-size partial blocks) are reported against the WebGL renderer / customGlyphs path when a TUI redraws every ~2s. Return: (1) the exact code-point ranges/lists handled by the custom-glyph path, quoted from xterm.js source; (2) whether braille U+2800–28FF and geometric shapes U+25A0–25FF (specifically ▰ U+25B0, ▱ U+25B1) are in that set or go through the font; (3) 2–4 GitHub issues describing stale/ghost/blank cells or broken block glyphs under `terminal.integrated.gpuAcceleration` on/auto with dates and status; (4) what the `gpuAcceleration` values mean today (is "canvas" gone?) and what `terminal.integrated.customGlyphs` controls.

## Answer

**▰ U+25B0 and ▱ U+25B1 are NOT custom-drawn — they go through the font**, because Geometric Shapes (U+25A0–U+25FF) appears in no definitions map and in no VS Code doc list. **Braille U+2800–U+28FF IS custom-drawn** as of VS Code 1.108 (Dec 2025), which expanded the set to ~800 characters. The custom-glyph code moved out of xterm.js core into the WebGL addon (`addons/addon-webgl/src/customGlyphs/`) in the 0.20.0-beta line, so anyone reading the old `src/browser/renderer/shared/CustomGlyphs.ts` is reading a stale file.

## The custom-glyph set

**Where the code lives today.** Not in core. The published core package `@xterm/xterm@6.1.0-beta.304` (the nightly VS Code bundles) contains **no file with "Glyph" in its name** — [jsDelivr flat file listing](https://data.jsdelivr.com/v1/packages/npm/@xterm/xterm@6.1.0-beta.304?structure=flat). The implementation is in `@xterm/addon-webgl@0.20.0-beta.300` at [`/src/customGlyphs/CustomGlyphDefinitions.ts`, `/src/customGlyphs/CustomGlyphRasterizer.ts`, `/src/customGlyphs/Types.ts`](https://data.jsdelivr.com/v1/packages/npm/@xterm/addon-webgl@0.20.0-beta.300?structure=flat). That directory does not exist in the released `@xterm/addon-webgl@0.19.0` (404), confirming it is new in the 0.20.0-beta line.

**The authoritative range list** — identical in the [VS Code 1.108 release notes](https://code.visualstudio.com/updates/v1_108), the [Terminal Appearance docs](https://code.visualstudio.com/docs/terminal/appearance), and [microsoft/vscode#286021](https://github.com/microsoft/vscode/issues/286021):

> "Box Drawing (U+2500–U+257F), Block Elements (U+2580–U+259F), Braille Patterns (U+2800–U+28FF), Powerline Symbols (U+E0A0–U+E0D4, Private Use Area), Progress Indicators (U+EE00–U+EE0B, Private Use Area), Git Branch Symbols (U+F5D0–U+F60D, Private Use Area), Symbols for Legacy Computing (U+1FB00–U+1FBFF)"

**Quoted from the source file.** [`CustomGlyphDefinitions.ts`](https://cdn.jsdelivr.net/npm/@xterm/addon-webgl@0.20.0-beta.300/src/customGlyphs/CustomGlyphDefinitions.ts) carries `#region` headers with the ranges written in:

- `// #region Box Drawing (2500-257F)` — U+2500–U+2503 solid, U+2504–U+250B dashed, U+250C–U+254B box components, U+254C–U+254F dashed, U+2550–U+2551 double, U+2552–U+256C double/light components, U+256D–U+2570 arcs, U+2571–U+2573 diagonals, U+2574–U+257B half lines, U+257C–U+257F mixed weight
- `// #region Block elements (2580-259F)` — U+2580–U+2590 blocks, U+2591–U+2593 shades, U+2594–U+2595, U+2596–U+259F quadrants
- `// #region Powerline Symbols (E0A0-E0BF)` — plus extended keys through U+E0C7

The regions after Powerline (braille, progress, git branch, legacy computing) **could not be quoted directly** — every fetch of the file truncated mid-Powerline. Their existence is corroborated by [`Types.ts`](https://cdn.jsdelivr.net/npm/@xterm/addon-webgl@0.20.0-beta.300/src/customGlyphs/Types.ts), whose `CustomGlyphDefinitionType` enum has seven members: `SOLID_OCTANT_BLOCK_VECTOR`, `BLOCK_PATTERN`, `PATH_FUNCTION`, `PATH`, `PATH_NEGATIVE`, `VECTOR_SHAPE`, `BRAILLE`.

**The dispatch predicate is a plain map lookup — no range arithmetic.** From [`CustomGlyphRasterizer.ts`](https://cdn.jsdelivr.net/npm/@xterm/addon-webgl@0.20.0-beta.300/src/customGlyphs/CustomGlyphRasterizer.ts):

```ts
export function tryDrawCustomGlyph(
  ctx: CanvasRenderingContext2D, c: string, ...
): boolean {
  const unifiedCharDefinition = customGlyphDefinitions[c];
  if (unifiedCharDefinition) { ... return true; }
  return false;
}
```

A character not present as a literal key falls through to font rasterization. The same file defines `drawBrailleCharacter(ctx, pattern, ...)` (8-bit dot pattern), `drawBlockVectorChar` (octants), `drawPatternChar`, `drawPathDefinitionCharacter`, `drawPathNegativeDefinitionCharacter`, `drawVectorShape`, `drawPathFunctionCharacter`.

**Historical baseline for comparison** — the older, still-published [`@xterm/xterm@6.0.0` `CustomGlyphs.ts`](https://cdn.jsdelivr.net/npm/@xterm/xterm/src/browser/renderer/shared/CustomGlyphs.ts) had only four maps and **no braille at all**: `blockElementDefinitions` (U+2580–2590, 2594–2595, 2596–259F, plus U+1FB70–1FB97 subset), `patternCharacterDefinitions` (U+2591–2593), `boxDrawingDefinitions` (U+2500–257F subset), `powerlineDefinitions` (exactly `\u{E0B0}`–`\u{E0BF}`), dispatched by `tryDrawCustomChar` in that order. If you are on an older VS Code, that narrower set is what you get.

## Braille and Geometric Shapes

- **Braille U+2800–U+28FF: custom-drawn.** Named in the 1.108 release notes and docs; backed by `BRAILLE` in the type enum and `drawBrailleCharacter` in the rasterizer. *Caveat:* I could not quote the literal U+2800-range keys from the definitions file due to fetch truncation — they may be populated programmatically rather than written out.
- **Geometric Shapes U+25A0–U+25FF: font path.** Absent from the definitions file (explicitly checked: no `25B0`, no `25B1`, no key in U+25A0–25FF beyond the block-element boundary at U+259F), absent from the rasterizer, and absent from every VS Code doc/release-note list. **▰ and ▱ are rasterized from your terminal font**, which means they inherit whatever cell-fit, baseline, and atlas behavior that font has — the exact class of problem custom glyphs were introduced to eliminate.

## Reported defects on the WebGL path

| Issue | Opened | Status | Symptom |
|---|---|---|---|
| [xtermjs/xterm.js#3303](https://github.com/xtermjs/xterm.js/issues/3303) | 2021-04-08 | **still open** | "Looks like remnants of earlier paint are there in webgl" — ghost text / stale cell content with ligatures; canvas renderer correct, WebGL wrong |
| [xtermjs/xterm.js#4665](https://github.com/xtermjs/xterm.js/issues/4665) | 2023-08-11 | closed (5.3.0 milestone) | "webgl regression: may not render as typing" — stale, non-updated cells; maintainer: "Will need to review recent changes/optimizations to webgl" |
| [microsoft/vscode#163936](https://github.com/microsoft/vscode/issues/163936) | 2022-10-18 | closed | "garbled with multiple cursor block characters and spaces randomly all over the terminal and portions of text disappear/reappear on terminal input/output"; only `gpuAcceleration: "off"` usable |
| [microsoft/vscode#204591](https://github.com/microsoft/vscode/issues/204591) | 2024-02-07 | closed as duplicate, flagged upstream | GPU-only cell-metric regression: "the text is placed too low and the descenders … are cut off", "the top of cursor block sticks out quite high" — wrong-size/misplaced blocks under `gpuAcceleration: auto` |

**Mechanism worth knowing for a ~2s-redraw TUI.** [`TextureAtlas.ts`](https://cdn.jsdelivr.net/npm/@xterm/addon-webgl@0.20.0-beta.300/src/TextureAtlas.ts) gates the custom path on `if (this._config.customGlyphs !== false)` and has an `_evictAllPages()` path triggered when page count exceeds `TextureAtlas.maxAtlasPages` or on `clearTexture()`. Eviction bumps `_pageLayoutVersion` to invalidate renderer models; the hazard is other owners still holding texture coordinates into wiped rows. A TUI cycling many distinct glyph/color combinations is exactly what pushes an atlas to eviction. *This mechanism-to-symptom link is my inference from reading the file, not a claim any cited issue makes — treat it as a hypothesis, not a diagnosis.*

Note: [microsoft/vscode#238417](https://github.com/microsoft/vscode/issues/238417) (2025-01-21, closed as duplicate, "GPU rendering causes ghost text to appear like normal text") surfaces on ghost-text searches but is labeled `editor-gpu` — it is the **editor's** GPU renderer and inline suggestions, **not** the terminal. Excluded from the table.

## The two settings today

- **`terminal.integrated.gpuAcceleration`**: valid values are **`"auto"` (default) and `"off"`** — [Terminal Appearance](https://code.visualstudio.com/docs/terminal/appearance): "The default terminal.integrated.gpuAcceleration value of `auto` tries the WebGL renderer and if it failed will fall back to the DOM renderer." Two renderers remain: "WebGL renderer - True GPU acceleration" and "DOM renderer - A fallback renderer that's much slower but has great compatibility."
- **`"canvas"` is gone.** Deprecated in [v1.89](https://code.visualstudio.com/updates/v1_89) and removed in [v1.90](https://code.visualstudio.com/updates/v1_90): "The canvas renderer was deprecated in the VS Code 1.89 release and is now removed completely." Older issues (e.g. #163936) still mention `canvas` because they predate removal. The value `"on"` in the original question is not a documented current value — docs list only `auto` and `off`; whether `"on"` is still silently accepted is **unverified**.
- **`terminal.integrated.customGlyphs`**: default `true`. "When GPU acceleration is enabled, custom rendering, rather than the font, improves how some characters display in the terminal" over the seven ranges listed above; "This feature can be disabled by setting terminal.integrated.customGlyphs to `false`." It only has effect when the WebGL renderer is active — it lives in the WebGL addon's `TextureAtlas`, so with `gpuAcceleration: off` (DOM renderer) it is inert.

## Open questions

1. **The literal braille / progress / git-branch / legacy-computing keys were never quoted.** Every fetch of `CustomGlyphDefinitions.ts` truncated mid-Powerline region. The ranges are asserted by VS Code's own docs and release notes and by the `BRAILLE` enum member, but I did not see `'\u{2800}'`-style keys with my own eyes. To close this, clone the repo and read `addons/addon-webgl/src/customGlyphs/CustomGlyphDefinitions.ts` directly.
2. **Powerline coverage disagrees between doc and source.** Docs say U+E0A0–U+E0D4; the source regions I could read stop at U+E0C7. Either the file continues past the truncation point or the docs overstate coverage.
3. **No issue in the four found is specifically about periodic-redraw TUIs.** The stale-cell reports are keystroke- and ligature-driven. Whether a ~2s full-frame repaint hits the same atlas-eviction path is untested here.
4. **No issue found reports a defect in the newly expanded (1.108) glyph set.** #286021, the test issue, closed clean. Absence of reports is not evidence of correctness — the expansion is only ~9 months old as of this writing.
5. **Practical consequence for ▰/▱:** since they take the font path, any cell-fit or ghosting artifact you see on them is a font-rasterization/atlas issue, not a custom-glyph issue — and `customGlyphs: false` will not change their behavior. Substituting a block-element or legacy-computing character (e.g. U+2580–259F or U+1FB00–1FBFF) would move them onto the pixel-perfect custom path.
