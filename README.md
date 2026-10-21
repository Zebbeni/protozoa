# Protozoa

A simulation of organisms evolving in a 2D environment. Each organism carries
inherited traits, a distribution of ability scores, and a brain — either a
decision tree or a feed-forward neural network — that picks one action per
cycle. Organisms that survive long enough reproduce, passing on slightly
mutated copies of all three.

Written in Go, rendered with [ebitengine](https://github.com/hajimehoshi/ebiten).
Runs on the desktop or in a browser via WebAssembly.

![Screen Shot 2022-07-30 at 12 33 53 AM](https://user-images.githubusercontent.com/3377325/181996580-b8a51e8c-0310-43ba-bcb5-3f189a59486b.png)

## Documentation

| Document | What it covers |
|---|---|
| **[ARCHITECTURE.md](ARCHITECTURE.md)** | How the program is put together: packages, the update cycle, the replay format, the rendering pipeline. Start here to work on the code. |
| **The in-app Rules screen** | What the simulation actually does — the environment, abilities, actions, both brain types. Reached from the main menu. It is drawn from the code and covered by tests, so it is the description that cannot go stale. |
| `CLAUDE.md` | Balance findings, measurements and the reasoning behind specific settings. Long, and organised by topic rather than read front to back. |
| `resources/README.md` | The sprite pipeline: the Aseprite source, the export script and the naming vocabularies it expects. |

## What is in the world

- **A wraparound grid.** Every cell holds a pH value between 0 and 10, which
  diffuses between neighbours each cycle. pH is the main environmental
  pressure: water far from an organism's ideal pH costs it health, and
  chemosynthesis only pays inside a band around that ideal.
- **Food**, on the surface and buried under it. Surface food is eaten; buried
  food has to be dug up first. Food settles out of reach over time, and corpses
  drop food where an organism died.
- **Walls**, with a strength from 1 to 100. They slow pH diffusion in
  proportion to their strength, which makes them shelter. The default
  `initial_walls` is 0 — every wall in a run is one an organism dug or burrowed.

## What an organism is

- **Seven ability scores** — Chemosynthesis, Eating, Movement, Digging, Attack,
  Defense, Tolerance — distributed across a fixed budget of 20 points, each
  capped at 10. A mutation moves exactly one point, so the budget buys about
  two specialisations and every point gained is a point given up.
- **Traits**: two colours, max size, spawn health, the health threshold for
  spawning, the gap between spawns, and ideal pH.
- **A brain**, one of two kinds, decided for the whole world by
  `use_neural_nets`:
  - a **decision tree** of conditions and actions, mutated by swapping,
    growing and pruning branches;
  - a **neural network** reading the same conditions and choosing between the
    same actions, mutated by drifting weights and by occasional changes to its
    shape.
- **Nine actions**: Chemosynthesis, Eat, Move, Turn Left, Turn Right, Attack,
  Dig, Idle, Spawn. Every organism can perform every action; the ability scores
  decide how effective each one is and what it costs.

Clicking an organism shows it in the panel across four tabs — TRAITS, STATS,
the brain, and its descendant tree — along with a per-cycle ledger of what
every health change cost or paid it.

## Setup

```bash
go get
go run .
```

## Run options

```bash
go run . -config settings/small.json   # load a settings file, skip the config screen
go run . -seed 2                       # fix the random seed
go run . -debug                        # memory and FPS readout
go run . -headless                     # no GUI
go run . -headless -trials 10          # several runs in sequence
go run . -dump-config                  # print the default settings as JSON
```

Settings files live in `settings/`. `-dump-config` prints a complete one to
paste and edit; the config screen can also save what you have edited under a
name of your own.

## Recordings

Every run writes a `.pzr` replay file. By default it goes to `protozoa_last.pzr`
in the system temp directory and is **overwritten by the next run**, so a run
worth keeping has to be saved out of the replay viewer's menu
(**Save Recording As...**), which copies it into `recordings/`.

```bash
go run . -checkpoint runs/big_mutation.pzr   # record to a chosen path
go run . -replay runs/big_mutation.pzr       # open a recording
go run . -resume                             # reopen the last run's recording
```

A recording stores the full settings it ran under in its header, so it replays
as the world it recorded rather than as whatever settings are currently active.
The viewer can play, pause, step a cycle either way, seek anywhere in the run,
and change speed.

## Run in a browser (WebAssembly)

Sprites, fonts, music and `settings/default.json` are bundled into the binary
with `go:embed`, so the wasm build needs no separate asset hosting.

```bash
./web/build.sh                              # writes web/protozoa.wasm and web/wasm_exec.js
python3 -m http.server 8080 --directory web # then open http://localhost:8080/
```

Notes and current limitations:

- The browser gets a smaller world — an 80x64 grid against the desktop's
  100x80, with the per-world item counts scaled down by the same area share.
  Exactly four of the 172 settings differ, and those are the four; everything
  else is the desktop's. See `config/browser_defaults.go`.
- The in-browser file system is virtual, so a recording does not survive a page
  reload. The replay controls work within a session; `-resume` and `-replay` are
  desktop-only.
- CLI flags do not apply — the browser passes no `os.Args`, so the wasm build
  always boots with the defaults.
- The binary is about 27 MB, mostly the Go runtime and the embedded assets.
  First load is the cold cost.

## Releasing

```bash
./dist.sh                 # builds both, stages dist/web and dist/windows
./push.sh <user>/<game>   # uploads both channels to itch.io with butler
```

## Tests

```bash
./test.sh                      # the whole tree
./test.sh ./manager/ -run Kill # arguments pass through
```

Use `./test.sh` rather than `go test ./...`: Go's default timeout is 10 minutes
per package and the `simulation` package does not fit in it. The script is
`go test -timeout 40m`.

Several tests are environment-gated measurement probes that assert nothing —
they exist so a balance question can be measured rather than guessed. They are
named `zz_*_test.go`, and each one's header comment gives its invocation.
