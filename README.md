# kbvmaker

kbvmaker composes a video from still images and a soundtrack. Motion, cuts, and dissolves are described in a Montage document. The engine resolves that document into a timeline and drives ffmpeg.

The application is kbvmaker. The file format is Montage. A usual project file is `name.montage.json`.

## Status

Montage 1 and the Go engine that loads it are in this tree. The command line can print the resolved plan, export a bash script, export the same commands as JSON, or render.

A windowed interface is later work. Its shape is in `IDEAS.md` under Front end. The resolution rules stay in the `v1` packages.

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

## Commands

```
go build -o kbvmaker ./cmd/kbvmaker

kbvmaker plan          file.montage.json
kbvmaker export-script file.montage.json
kbvmaker export-plan   file.montage.json
kbvmaker render        file.montage.json
```

`plan` prints the resolved timeline. `export-script` and `export-plan` print ffmpeg argument lists, as bash and as a `renderPlan` JSON document. `render` runs those commands with the Montage file's directory as the working directory and writes intermediate files under `work/`.

## Layout

The module path is `neal/kbvmaker`. Montage 1 is the package tree `v1`.

```
Document → Plan → []Command → bash | renderPlan JSON | exec
```

| Package | Responsibility |
| --- | --- |
| `v1` | Facade: load, resolve, compile, export, run. |
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
```

Unit tests cover decode, clocks, fill zoom, scheduling, and the 20-second two-clip timing case. One test resolves a document through a fake probe and checks that both exports carry the compiler's argv. One test writes a PNG and a WAV, renders a short hold, and checks the output size and duration with ffprobe.
