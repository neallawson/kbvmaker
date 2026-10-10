# kbvmaker

kbvmaker composes a video from still images and a soundtrack. Motion, cuts, and dissolves are described in a Montage document. The engine resolves that document into a timeline and drives ffmpeg.

The application is kbvmaker. The file format is Montage. A usual project file is `name.montage.json`.

## Status

Montage 1 and the Go engine that loads it are in this tree. The command line can print the resolved plan, export a bash script, export the same commands as JSON, or render.

The window and the local browser are two hosts of one Svelte client. Both call the Go API at `/v1`. The editor creates a Montage, lists the pictures, and shows a read-only score of the plan. The shape is in `IDEAS.md` under Editor and Hosts. A multi-session site is later. The resolution rules stay in the `v1` packages.

`make_Greenhouse_video_v3.sh` is the first Greenhouse tape, kept as a reference. It is not the application.

## Documents

| File | Role |
| --- | --- |
| `montage-v1.md` | Normative specification of Montage 1. |
| `IDEAS.md` | Design record: why the frame model, timing, and exports look the way they do. |
| `AGENT.md` | Notes for an agent changing this repository. |

Where `IDEAS.md` and `montage-v1.md` disagree, the specification wins.

## Requirements

Go 1.23. ffmpeg and ffprobe on `PATH` for probing media and for `render`. `go test ./...` skips the short render when ffmpeg is absent.

The window on Linux needs WebKitGTK and python3 with GTK 3 (`python3-gi`) for the file dialogs. Releases target Ubuntu 22.04 or newer with `libwebkit2gtk-4.1-dev`. Older systems with only `libwebkit2gtk-4.0-dev` can build the window for development.

## Commands

```
go build -o kbvmaker ./cmd/kbvmaker

kbvmaker plan          file.montage.json
kbvmaker export-script file.montage.json
kbvmaker export-plan   file.montage.json
kbvmaker render        file.montage.json
kbvmaker serve

pnpm --dir frontend build
go build -tags production,webkit2_41 -o kbvmaker-window ./cmd/desktop   # WebKitGTK 4.1
go build -tags production -o kbvmaker-window ./cmd/desktop              # WebKitGTK 4.0
```

The generated ffmpeg commands print only errors and the progress line. Pass `-verbose` before the file (for example `kbvmaker render -verbose file.montage.json`) to keep ffmpeg's banner and warnings. `plan` prints the resolved timeline. `export-script` and `export-plan` print ffmpeg argument lists, as bash and as a `renderPlan` JSON document. `render` runs those commands with the Montage file's directory as the working directory and writes intermediate files under `work/`. `serve` listens on `127.0.0.1:8787` and exposes `/v1`. Pass `-addr` to change the address, and `-static frontend/build` to serve the client from the same origin. `kbvmaker-window` opens that client in a Wails window and adds a native file dialog. Build the client first.

## Layout

The module path is `neal/kbvmaker`. Montage 1 is the package tree `v1`.

```
Document → Plan → []Command → bash | renderPlan JSON | exec
```

| Package | Responsibility |
| --- | --- |
| `v1` | Facade: load, resolve, compile, export, run. |
| `web/v1` | HTTP API for Montage 1. |
| `frontend/` | Svelte client shared by the browser and the window. |
| `v1/version` | The integer shared by the API, `montage`, and `renderPlan`. |
| `v1/doc` | JSON types, defaults, and structural checks. |
| `v1/model` | The resolved plan. |
| `v1/media` | Probe interface. ffprobe is one implementation. |
| `v1/clock` | Time spellings resolved to seconds. |
| `v1/frame` | Fill zoom, fitted rectangle, clamped window. |
| `v1/schedule` | Pools, natural sort, shuffle, and loop. |
| `v1/timing` | `fit-audio`, `fixed-clip`, `explicit`, `markers`. |
| `v1/motion` | Built-in modes and segment expansion. |
| `v1/check` | Rule checks on a resolved slot. |
| `v1/plan` | Orchestrator. |
| `v1/compile` | Plan to ffmpeg argv, including batched `xfade`. |
| `v1/export` | Bash and `renderPlan` JSON. |
| `v1/run` | Exec the argv list. Stop on the first failure. |
| `v1/rule` | Slot index, stable code, message. |

`v1.Version`, the document field `montage`, and the export field `renderPlan` are the same number. Today that number is 1. A later Montage is a new package, `v2`, which accepts `montage: 2`. Version 1 keeps loading version-1 documents.

## Tests

```
go test ./...
pnpm --dir frontend test
```

Unit tests cover decode, clocks, fill zoom, scheduling, and the 20-second two-clip timing case. One test resolves a document through a fake probe and checks that both exports carry the compiler's argv. One test writes a PNG and a WAV, renders a short hold, and checks the output size and duration with ffprobe.
