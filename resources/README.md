# Sprite authoring & export

All in-game sprites are authored in a single Aseprite file and exported as
per-resolution PNG strips by `gen/export_spritesheets.lua`. The script
walks every `(slice, layer, tag)` combination in the file, hides every
other drawable layer for each export so layers come out isolated, and
slices the right number of frames out of each tag based on the slice's
resolution.

The renderer then composites at draw time: under-body overlays first,
then the body variant matching the organism's defense feature, then
over-body sensor overlays on top.

## One file, all resolutions

Every sprite — every organism size, every food size, walls, every body
variant, every feature overlay — lives in **one** `.aseprite` file.
Slice names carry the resolution, so the same file can hold `small_4`,
`small_16`, and `small_32` slices side by side. The script handles the
rest.

## Slices

A slice is a rectangle in the source file that maps to one cell (or one
2-cell sprite). Name pattern:

```
<stem>_<res>            animated 1-cell  (e.g. small_16)
<stem>_<res>_xl         animated 2-cell  (e.g. small_16_xl)
<stem>_<res>_static     static, paired only with the `base` tag
                        (e.g. food_small_16_static, wall_32_static)
```

Stems the renderer expects:

| Slice stem | Use |
|---|---|
| `small`, `medium`, `large` | Organism size buckets (thirds of `MaximumMaxSize`) |
| `food_small`, `food_medium`, `food_large` | Food, by value-bucket; always `_static` |
| `wall_weak`, `wall_medium`, `wall_strong` | Wall, by strength tier; always `_static`. Renderer picks the tier from the cell's current strength: 1–2 → weak, 3–5 → medium, 6–7 → strong |

Resolutions: `4`, `8`, `16`, `32`. Authoring at every resolution is
optional — only the ones you create slices for will be exported.

## Tags

**One tag per action**, regardless of resolution. The script slices off
the right number of frames per export:

| Slice resolution | Frames taken from the tag |
|---|---|
| 4x4 | first 1 frame |
| 8x8 | first 2 frames |
| 16x16 | first 4 frames |
| 32x32 | first 8 frames |

A tag with fewer frames than a slice demands is clamped — the animation
just plays however many you drew, in place of the full count. Useful
when you're prototyping: start every tag at 1 frame, expand to 8 later.

Tag names — `<action>` for 1-cell, `<action>_xl` for 2-cell, plus a
single `base` tag for static art:

| Tag | Status it routes to |
|---|---|
| `idle` | StatusIdle / fallback for any status without a dedicated animation |
| `move` | StatusMoveSuccess, birth frames |
| `blocked` | StatusMoveBlocked |
| `turn_left` | StatusTurnLeft |
| `turn_right` | StatusTurnRight |
| `attack` | StatusAttacking (today also StatusStinging) |
| `eat` | StatusEatSuccess |
| `eatfail` | StatusEatFailed |
| `chemo` | StatusChemoSuccess |
| `chemofail` | StatusChemoFailed |
| `die` | StatusDying |
| `sting` | StatusStinging (pending — not yet wired in `ForStatus`) |
| `dig` | StatusDigging (pending) |
| `burrow` | StatusBurrowing (pending) |
| `hunker` | StatusHunkering (pending) |
| `flare` | StatusFlaring (pending) |
| `hide` | StatusHiding (pending) |
| `spawn` | StatusSpawning (pending) |
| `base` | Static, paired only with `_static` slices |

The "pending" tags can be authored now; they'll produce PNGs that sit
unused until `animation.ForStatus` is extended to route those statuses
to dedicated animations.

## Layers

Layer choice determines which exports a slice produces. The script hides
every other drawable layer before each export so each PNG ends up
isolated, ready for the renderer to composite at draw time.

### Drawable layers

**Low-res organism body** (used only with `small_4`, `small_8`,
`medium_4`, `medium_8`, `large_4`, `large_8` slices):

- `body`

**High-res organism body variants** (used only with `small_16`,
`small_32`, `medium_16`, `medium_32`, `large_16`, `large_32` slices —
all four are exported for every size × action):

- `body_basic`
- `body_shell`
- `body_spikes`
- `body_camouflage`

**High-res feature overlays** (same slices as the body variants). The
renderer draws them in three z-tiers around the body silhouette so
parts that should poke past the body stay visible:

Drawn *under* the body — these attach to or extend out from the body's
sides / rear and the body silhouette covers their roots cleanly:

- `flagellae`
- `cilia`
- `stinger`
- `teeth`
- `fangs`
- `tusks`

Drawn *over* the body — head-mounted sensors that need to read as
"sticking out in front of" the silhouette, not hidden underneath:

- `antennae`
- `feelers`
- `tasters`

Compositing order, bottom to top:
1. Under-body overlays (flagellae, cilia, stinger, teeth, fangs, tusks)
2. Body variant (basic / shell / spikes / camouflage — exactly one)
3. Over-body overlays (antennae, feelers, tasters)

**Static art** (any resolution; pairs only with the matching `_static`
slice stem):

- `food`
- `wall`

### Preview-only layers (never exported, even when visible)

- `background` — a backdrop you can use to see how your sprites read
  in-context while drawing. The script hides it for every export
  regardless of its current visibility.
- Any layer whose name starts with `_` or `.` — treated as drafts /
  references / guides.

## Slice-to-layer mapping

The script picks which layers to export against each slice from the
slice's stem and resolution:

| Slice stem | Resolution | Layers exported |
|---|---|---|
| `small`, `medium`, `large` | 4, 8 | `body` |
| `small`, `medium`, `large` | 16, 32 | `body_basic`, `body_shell`, `body_spikes`, `body_camouflage`, plus all 9 feature overlays |
| `food_*` | any | `food` |
| `wall_*` | any | `wall` |

Any other stem is treated as an organism stem.

## Output filenames

```
<outdir>/<res>x<res>/<filename>
```

Filename rules:

| Slice → | Filename |
|---|---|
| Animated, single applicable layer (low-res organism) | `<stem>_<action>.png` — e.g. `small_idle.png` |
| Animated, multiple applicable layers (high-res organism) | `<layer>_<stem>_<action>.png` — e.g. `body_basic_small_idle.png`, `flagellae_small_move.png` |
| Static (food, wall) | `<stem>.png` — e.g. `food_small.png`, `wall.png` |

The layer prefix is only added when more than one layer applies, so the
existing 4x4 / 8x8 outputs stay backwards-compatible.

## Running the script

1. Open the source `.aseprite` file in Aseprite.
2. `File` → `Scripts` → `Run Script...` and pick
   `resources/gen/export_spritesheets.lua`.
3. Point the **Output folder** entry at `resources/images/grid` (the
   default suggestion is the directory the source file lives in).
4. Hit **Export**.

The script creates `4x4/`, `8x8/`, `16x16/`, `32x32/` subdirectories
under the output folder as needed and writes one PNG per
`(slice, applicable layer, matching tag)` triple. It then reports the
count and prints three sanity-check sections in the final alert when
relevant:

- **Missing (expected but not found)** — slices, tags, or layers
  named in the canonical set that aren't in the file. For each
  resolution that has *any* slice, the script expects the canonical
  organism / food / wall stems at that resolution; for any animated
  slice it expects the 11 wired animated action tags; for any `_xl`
  slice it expects the matching `_xl` tag variants; for any static
  slice it expects the `base` tag; layer expectations depend on
  which resolutions are in use.
- **Unrecognised** — drawable layers whose names aren't in the
  project's known layer set, and aren't preview layers (`background`
  or `_`-prefixed). Almost always a typo.
- **Skipped** — slices / tags whose names don't match the script's
  naming patterns at all (parse failures).

The export still runs for everything that *can* be exported even when
warnings are present, so you can iterate on missing art / typos
without re-running the dialog.

Under the hood it saves a scratch copy of your file, crops the scratch
to each slice's bounds, toggles visibility on the scratch copy, and
runs Aseprite's built-in `ExportSpriteSheet` to produce a
horizontal-strip PNG. The scratch copy is closed and deleted at the
end, so your source file's contents, slice geometry, and layer
visibility are untouched.

## Worked example

A sprite file with these slices, tags, and layers:

- Slices: `small_4`, `small_16`, `food_small_16_static`, `wall_weak_16_static`, `wall_medium_16_static`, `wall_strong_16_static`
- Tags: `idle` (4 frames), `move` (8 frames), `attack` (8 frames), `base` (1 frame)
- Layers: `background` (preview), `body`, `body_basic`, `body_shell`,
  `body_spikes`, `body_camouflage`, `flagellae`, `cilia`, `stinger`,
  `antennae`, `feelers`, `tasters`, `teeth`, `fangs`, `tusks`, `food`, `wall`

…produces these PNGs:

```
4x4/small_idle.png             (body × idle, first 1 frame)
4x4/small_move.png             (body × move, first 1 frame)
4x4/small_attack.png           (body × attack, first 1 frame)

16x16/body_basic_small_idle.png       (body_basic × idle, first 4 frames)
16x16/body_basic_small_move.png       (body_basic × move, first 4 frames)
16x16/body_basic_small_attack.png
16x16/body_shell_small_idle.png
16x16/body_shell_small_move.png
...                                   (all body × size × action combos)
16x16/flagellae_small_idle.png        (flagellae × idle, first 4 frames)
16x16/flagellae_small_move.png
...                                   (all overlay × size × action combos)
16x16/food_small.png                  (food × base, 1 frame)
16x16/wall_weak.png                   (wall × base, 1 frame)
16x16/wall_medium.png
16x16/wall_strong.png
```

`background` is hidden for every export, so even if it's visible in
Aseprite you never see it in the PNGs.
