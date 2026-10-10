# kbvmaker

kbvmaker composes a video from still images and a soundtrack. Motion, cuts, and dissolves are described in a Montage document. The engine resolves that document into a timeline and drives ffmpeg.

The application is kbvmaker. The file format is Montage. A usual project file is `name.montage.json`.

## Status

Montage 1 and the Go engine that loads it are in this tree. The command line checks a Montage, prints the resolved plan, exports the render as a bash script or as `renderPlan` JSON, and renders the video. It is built to run unattended in scripts and pipelines. It reads and renders Montage files but does not create or edit them yet.

The window and the local browser are two hosts of one Svelte client. Both call the Go API at `/v1`. The editor creates and saves a Montage, lists the pictures in their saved order, pins picture lengths, applies looks, shows a read-only score of the plan, and exports a script or renders the video. The shape is in `IDEAS.md` under Editor and Hosts. A multi-session site is later. The resolution rules stay in the `v1` packages.

`make_Greenhouse_video_v3.sh` is the first Greenhouse tape, kept as a reference. It is not the application.

## Documents

| File | Role |
| --- | --- |
| `montage-v1.md` | Normative specification of Montage 1. |
| `IDEAS.md` | Design record: why the frame model, timing, and exports look the way they do. |
| `CLI.md` | Command-line reference: commands, streams, exit codes, overrides, rendering. |
| `AGENT.md` | Notes for an agent changing this repository. |

Where `IDEAS.md` and `montage-v1.md` disagree, the specification wins.

## Requirements

Go 1.23. ffmpeg and ffprobe on `PATH` for probing media and for `render`. `go test ./...` skips the short render when ffmpeg is absent.

The window on Linux needs WebKitGTK, which brings GTK 3. The file dialogs are native GTK, built with cgo against the same library, so nothing else is required. Windows and macOS use the Wails dialogs. Releases target Ubuntu 22.04 or newer with `libwebkit2gtk-4.1-dev`. Older systems with only `libwebkit2gtk-4.0-dev` can build the window for development.

## Command line

```
go build -o kbvmaker ./cmd/kbvmaker

kbvmaker validate      [--media] FILE    check the document (--media: also the song, photos, ffmpeg)
kbvmaker plan          FILE              resolved timeline as JSON
kbvmaker export-script FILE              the render as a bash script
kbvmaker export-plan   FILE              the render as renderPlan JSON
kbvmaker render        FILE              run ffmpeg and write the video
kbvmaker serve                           the /v1 HTTP API on 127.0.0.1:8787
kbvmaker version
kbvmaker help [COMMAND]
```

`FILE` is a Montage document, or `-` to read one from stdin (relative paths then resolve against `-dir`). Flags may come before or after it. `CLI.md` is the full reference.

Everyday use:

```sh
kbvmaker validate --media piece.montage.json          # ok: 24 pictures, 3:42
kbvmaker render piece.montage.json                    # [5/30] clip 3/24 ... wrote /films/piece/out.mp4 in 4:12
kbvmaker render -o take2.mp4 --audio take2.wav piece.montage.json
kbvmaker export-script -o render.sh piece.montage.json && ./render.sh
```

### In a pipeline

The command line keeps one contract so a script can rely on it:

- **stdout is data only:** plan JSON, the script, the `renderPlan`, the rendered video's path, or `ok`. `-o FILE` writes the result to a file instead, through a temporary file and a rename.
- **stderr carries progress and errors.** `-json` prints an error as one line, `{"slot":2,"code":"missing-media","message":"image b.jpg"}`, the same shape as `/v1`. `render -progress json` prints one JSON object per step.
- **Exit codes say what failed:**

  | Code | Meaning |
  | --- | --- |
  | 0 | Success |
  | 1 | Unexpected failure |
  | 2 | Usage mistake |
  | 3 | Document breaks a rule |
  | 4 | Missing media or tool |
  | 5 | ffmpeg step failed |
  | 130 | Interrupted |

- **Flags override the file for one run.** `-audio`, `-join`, `-hold`, `-zoom-span`, `-blur`, `-seed`, `-schedule`, and `-o` (the video, on `render`) win over the document. Nothing is written back.
- **Renders are safe to retry.** The video is written under a hidden partial name and renamed at the end, so a failed or interrupted run never leaves half a video at the output path. SIGINT or SIGTERM stops ffmpeg. `-work DIR` moves the intermediates, and `-clean` removes them after success.

```sh
#!/bin/sh
# Render every project in a folder. Skip broken ones, stop if ffmpeg breaks.
for f in projects/*.montage.json; do
  if ! kbvmaker validate --media --json "$f" 2>>errors.jsonl; then
    echo "skip $f" >&2
    continue
  fi
  video=$(kbvmaker render --clean --progress json "$f" 2>>progress.jsonl)
  status=$?
  case $status in
    0)   echo "$video" >> rendered.txt ;;
    5)   echo "ffmpeg failed on $f" >&2; exit 5 ;;
    130) exit 130 ;;
    *)   echo "failed $f ($status)" >&2 ;;
  esac
done
```

```sh
# A generated document on stdin, rendered against a media folder.
generate-montage | kbvmaker render --dir /media/greenhouse -o /out/greenhouse.mp4 -
```

`serve` exposes the same engine over HTTP for programs that prefer it. `-addr` changes the address, and `-static frontend/build` also serves the client. It stops cleanly on SIGINT or SIGTERM.

## Window and browser

```
pnpm --dir frontend build
go build -tags production,webkit2_41 -o kbvmaker-window ./cmd/desktop   # WebKitGTK 4.1
go build -tags production -o kbvmaker-window ./cmd/desktop              # WebKitGTK 4.0
./kbvmaker-window

kbvmaker serve -static frontend/build                                   # the same client in a browser
```

The window opens the client with native file dialogs that reopen in the last folder used. The browser host types paths instead. Both reach the engine through `/v1`. Build the client before the window, and restart the window after rebuilding the client.

## Layout

The module path is `neal/kbvmaker`. Montage 1 is the package tree `v1`.

```
Document → Plan → []Command → bash | renderPlan JSON | exec
```

| Package | Responsibility |
| --- | --- |
| `v1` | Facade: load, resolve, compile, export, run. |
| `web/v1` | HTTP API for Montage 1. |
| `cmd/kbvmaker` | Command line. Calls the `v1` facade only. |
| `cmd/desktop` | Wails window: native dialogs and the `/v1` bridge. |
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
| `v1/run` | Exec the argv list with progress events, an atomic final write, and cancellation. Stop on the first failure. |
| `v1/rule` | Slot index, stable code, message. |

`v1.Version`, the document field `montage`, and the export field `renderPlan` are the same number. Today that number is 1. A later Montage is a new package, `v2`, which accepts `montage: 2`. Version 1 keeps loading version-1 documents.

## Tests

```
go test ./...
pnpm --dir frontend test
```

Unit tests cover decode, clocks, fill zoom, scheduling, and the 20-second two-clip timing case. One test resolves a document through a fake probe and checks that both exports carry the compiler's argv. One test writes a PNG and a WAV, renders a short hold, and checks the output size and duration with ffprobe.

The command-line tests run `cli` with a fake probe and a stand-in ffmpeg script. They cover help, every usage error, exit codes, JSON errors, stdin and `-dir`, overrides, `-o` files, progress, `-clean`, a failed step, an interrupt, `serve`, and a plan that matches `/v1`. One more test renders a real video when ffmpeg is installed. The desktop tests open the real GTK dialogs when a display is available, and send an editor save through the window's `/v1` bridge. The frontend tests (Vitest with happy-dom) cover the document operations, the editor screen, and the window host.
