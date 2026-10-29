# kbvmaker - Video Maker for Still Images and a Soundtrack

kbvmaker makes a slideshow video from still images, title cards, credit cards, and a soundtrack. Motion, cuts, and dissolves are supported. The application drives ffmpeg to render the video. Title and credit cards are painted in Go and fed to ordinary ffmpeg; no special build is required.

The application is kbvmaker. The file format is Montage. A usual project file is `name.montage.json`.
The GUI is kbvmaker-gui. This is a front-end, GUI-based, local application for creating and rendering Montage files.

## Status

Montage 1 and the Go engine that loads it are in this tree. The command line checks a Montage, prints the resolved plan, exports the render as a bash script or as `renderPlan` JSON, and renders the video. It is built to run unattended in scripts and pipelines. It reads and renders Montage files but does not create or edit them yet.

The GUI and the local browser are two hosts of one Svelte client. Both call the Go API at `/v1`. The editor creates and saves a Montage, lists the pictures and cards in their saved order, pins lengths, applies looks, reorders and resizes slots on the timeline, and exports a script or renders the video. The shape is in `IDEAS.md` under Editor and Hosts. A multi-session site is later. The resolution rules stay in the `v1` packages.

## Documents

| File | Role |
| --- | --- |
| `montage-v1.md` | Normative specification of Montage 1. |
| `IDEAS.md` | Design record: why the frame model, timing, and exports look the way they do. |
| `CLI.md` | Command-line reference: commands, streams, exit codes, overrides, rendering. |
| `AGENT.md` | Notes for an agent changing this repository. |
| `README.md` | This document. |


Where `IDEAS.md` and `montage-v1.md` disagree, the specification wins.

## Install

Pushing a tag such as `v1.0.0` runs the Windows build and attaches `kbvmaker-windows-amd64.zip` to that GitHub release. The zip contains `kbvmaker.exe` and `kbvmaker-gui.exe`. Rendering needs `ffmpeg` and `ffprobe` on `PATH`. The editor uses the WebView2 runtime that current Windows 10 and Windows 11 already include. Linux and macOS are built from source.

The command line needs [Go 1.23](https://go.dev/dl/) or newer, plus `ffmpeg` and `ffprobe` on `PATH`. `go test ./...` skips the short render when ffmpeg is absent. The GUI needs those and a few more tools. `./scripts/build` checks for them and, on Linux, uses the WebKitGTK library that is already installed.

## Build from source

### Command line

```
go build -o kbvmaker ./cmd/kbvmaker
```

### Desktop editor

Linux and macOS:

```
./scripts/build
./kbvmaker-gui
```

Windows, from PowerShell:

```
powershell -File scripts\build.ps1
.\kbvmaker-gui.exe
```

Git Bash or WSL on Windows can run `./scripts/build` instead.

The script installs frontend dependencies on a fresh checkout, builds the Svelte client, and embeds that client in the GUI. The window serves the copy it was built with. Restart kbvmaker-gui after `./scripts/build`. On Linux it uses WebKitGTK 4.1 when that library is installed, and WebKitGTK 4.0 otherwise. If neither is installed, it prints one package command for apt, dnf, pacman, or zypper and stops. Windows uses the WebView2 runtime that ships with current Windows 10 and Windows 11. macOS uses the system WebKit, and the script asks for the Xcode command line tools when `clang` is missing.

## GUI

kbvmaker-gui is the editor. One screen holds the project, the song, a look, the pictures, and a timeline of how the film will play. The same screens run in a browser. The GUI adds native file dialogs that reopen in the last folder used for that kind of file.

The browser host types a path into the box instead of opening a dialog:

```
kbvmaker serve -static frontend/build
```

ffmpeg runs on the machine that serves the page.

![The editor: project, song, look, pictures, and the timeline](docs/gui.png)

### Making a film

**New** starts a Classic project in memory. **Save** asks where to write it, and the dialog suggests `piece.montage.json`. **Save** is filled in while the film has unsaved changes, and it is quiet and cannot be clicked once those changes are written. **Open** reads a Montage that already exists. The line beside the buttons shows "Unsaved changes" until Save, then "Saved". Closing the window, or the browser tab, asks before those changes are discarded. The path sits under the buttons. Save writes the file with no song and no pictures yet. Picture and song paths are stored relative to the project file, so the folder can move with its media.

The clock is two tabs. **Soundtrack** is open on a new project. **Add soundtrack** appends one audio file (`mp3`, `wav`, `m4a`, `flac`, or `aac`). Each file is a row: its name, and the silence before it when there is any. The length of the whole soundtrack sits beside the list. **Silence before** sets that quiet in seconds. **Move soundtrack up** and **Move soundtrack down** reorder the files, and so does dragging a block on the soundtrack lane. **Replace soundtrack** swaps the selected file. **Remove soundtrack** drops it. The last file, or **No Soundtrack**, clears the list. Choosing the first file does not change the clock until that file is picked. Classic or Photo film then makes the soundtrack the length, including the silences. Pictures keep playing through a silence. Quick slides stays at 5 seconds each, and the film ends with whichever is shorter, the pictures or the soundtrack. An opened file that is already on a picture clock keeps that clock. One file with no silence is still a one-element `tracks` list. There is no `audio` field.

**No Soundtrack** drops the song and keeps each picture's length. **Film length** is the finished video, shown the same way as the song length (`m:ss · Ns`). **Time for each picture** is the one field. A new project is 5 seconds each, so adding a picture grows the film. Dissolves overlap: ten pictures at 5 seconds with a 1 second dissolve make a 41 second film. A picture's own **Length in seconds** changes that one picture and leaves the others at the shared time. The readout is the sum after the overlaps.

A **look** sets the pacing, the dissolve, and a move on every picture. Classic is selected for a new project. The project file keeps its name. The video is 1920×1080.

| Look | How time is shared | Between pictures | Behind the photo | Moves |
| --- | --- | --- | --- | --- |
| Classic | Pictures share the song, or 5 seconds each when there is no song | 1s dissolve | Black | Alternates slow zoom in and slow zoom out |
| Photo film | Pictures share the song, or 5 seconds each when there is no song | 1.5s dissolve | A blurred copy of the photo | Cycles slow zoom in, pan right, and slow zoom out |
| Quick slides | Each picture is 5 seconds | 0.5s dissolve | Black | Slow zoom in on every picture |

Quick slides gives every picture the same five seconds, so a length pinned on one picture is cleared.

**Add pictures** appends photographs in the order you picked (`jpg`, `jpeg`, `png`, or `webp`). **Add card** appends a title card: the word Title, Hold still, white on black. That order is the film. **Sort by name**, **Move up**, and **Move down** rewrite it. **Pin position** keeps that slot's place through a shuffle or a sort, whether the slot is a photograph or a card. **Unpin position** lets it move again. A file that shows up later in a folder stays out of the film until you add it.

A card is a slot like a photograph: it has a length, a dissolve, and a move. The selected card has a title, an optional subtitle, an optional third line, and a credits list (one line per entry). Those fields update the card when you leave them. Placement is left, center, or right, and upper, middle, or lower. Size is small, medium, or large. Weight is regular or bold. Face is sans or serif, from fonts shipped with the app. Text color and background are bordered color controls, each showing its `#RRGGBB` value. An optional faint box sits behind the text. Hold still is the default so the letters stay sharp. Drift up is the credits roll. Replace is off on a card. Applying a Look, and Shuffle moves, leave cards on Hold still. The timeline shows the title and a thumbnail of the painted card. The Montage stores the text and the style. The PNG is written at render into `work/`, the same family of files as `canvas-N.png`.

The timeline shows each picture as soon as you add it, as a block on a film strip. Black rails along the top and bottom carry a repeating row of sprocket holes. The block shows a thumbnail of the picture or card, and its number when there is room. The file name, or the card's title, is the tooltip. The picture list is one line per picture until that picture is selected, and then the move, the length, and the pin open under the name. **Find a picture** jumps to a name. The blocks take their clip lengths, a darker band marks each dissolve, and the axis runs from 0:00 to the end of the film. Under the strip, the soundtrack lane draws each file as wide as that file is long, on the same axis. A grip at the start of a later file sets the silence before it. Before the length is known, the picture blocks are equal. The selected picture is outlined. Click a block or a row to select that picture. A click on a timeline block also opens a larger view of the whole picture. Click that view, or press Escape, to close it. Drag a block to reorder: the picture lifts off the strip and follows the pointer, and the other pictures slide apart to leave a gap where it will land. Drop it anywhere on the timeline, and the pictures' centers decide that gap. A drop on a center counts as the left side of that center. At or left of the first center, the picture becomes first. Right of the last center, it becomes last. The picture keeps its move and its length. A grip in the center of each dissolve changes the two neighboring lengths. The mark is a short vertical line, and the grab area around it is wider. That grab area leaves the center of each picture free, so a short picture can still be selected. Dragging the grip gives time to one picture and takes the same time from the other, the blocks follow the pointer, and the film's length stays. Releasing the pointer updates the open project. **Save** writes the file. One picture has no grip. When a picture would be too narrow to grab, both lanes scroll together and a **Film window** above them shows the whole film: picture ticks, soundtrack bars, and the part currently open. Dragging that window moves both lanes. Thumbnails are requested for the pictures in that window and for the rows on screen. The line under the timeline says how it will come out: how many pictures, about how long each one stays, and the dissolve between them.

Thumbnails come from `POST /v1/thumbnail` with `{path, width, height}` or `{card, width, height}`, exactly one of `path` or `card`. Each edge is from 1 to 512 pixels. A photograph is scaled to cover that box and center-cropped. A card is painted to that box. The reply is JSON: `{type, width, height, data}` where `data` is a base64 JPEG. The window and the browser both use that JSON.

The selected picture can use any move: Slow zoom in, Pan right, Pan left, Drift down, Drift up, Slow zoom out, or Hold still. Applying a Look still writes that Look's own cycle onto the photographs and leaves cards on Hold still. **Shuffle moves** draws from that cycle, which Save stores in the Montage. Cards stay on Hold still. Classic shuffles slow zoom in and slow zoom out. Photo film shuffles slow zoom in, pan right, and slow zoom out. Quick slides shuffles slow zoom in. When the pictures share a song and there are two or more, **Length in seconds** pins that picture and the others share what remains. With no song, that box gives one picture its own time and leaves the others at Time for each picture. **Remove custom time** clears that time. Dragging a dissolve grip stores those two lengths and keeps the film's length. One picture runs for the whole song. Quick slides starts with one length for every picture, so the length box stays closed. The first grip drag leaves that shared length: each picture keeps the length it has, and later drags change them one pair at a time. Applying Quick slides again restores the one shared length. **Replace** and **Remove** change that picture. **Shuffle pictures** reorders the slots that are not pinned in place. **Sort by name** does the same, and writes the names into the open indexes. **Shuffle moves** assigns each picture one of that Look's moves once. The next time the file opens, that order and those moves are what was saved.

**Export shell script** and **Render video** stay off until a plan can see every picture and, when the film uses one, the soundtrack. A film with no soundtrack is video only. A missing soundtrack, picture, or backdrop is listed above those buttons. **Re-scan Pictures/Audio**, with the picture controls, reads the files from disk and reloads the thumbnails. Returning to the window does the same check, so a file put back at its old path, or a new picture saved over the same name, shows up without a change to the Montage. Each action saves the Montage first, then asks where the file should go (`render.sh` or `out.mp4`). While a render runs, the controls lock and a timer counts beside the button. When it finishes, the line names the video and how long the render took. A failed ffmpeg step shows its error. An engine error is shown on the picture it belongs to.

Each dialog remembers its own folder: the project, the pictures, the song, the script, and the video.

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

## Layout

The module path is `kbvmaker`. [Montage 1](montage-v1.md) is the package tree `v1`.

```
Document → Plan → []Command → bash | renderPlan JSON | exec
```

| Package | Responsibility |
| --- | --- |
| `v1` | Facade: load, resolve, compile, export, run. |
| `web/v1` | HTTP API for Montage 1. |
| `cmd/kbvmaker` | Command line. Calls the `v1` facade only. |
| `cmd/desktop` | Wails GUI: native dialogs and the `/v1` bridge. |
| `frontend/` | Svelte client shared by the browser and the GUI. |
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
| `v1/card` | Paint a title or credits card to a PNG or JPEG. Liberation Sans and Serif are embedded. |
| `v1/compile` | Plan to ffmpeg argv, including batched `xfade`. A card slot is a PNG write plus a clip that reads it. |
| `v1/export` | Bash and `renderPlan` JSON. Card PNGs are base64 in the script and in `files`. |
| `v1/run` | Exec the argv list with progress events, an atomic final write, and cancellation. Write card PNG bytes. Stop on the first failure. |
| `v1/rule` | Slot index, stable code, message. |

`v1.Version`, the document field `montage`, and the export field `renderPlan` are the same number. Today that number is 1. A later Montage is a new package, `v2`, which accepts `montage: 2`. Version 1 keeps loading version-1 documents.

## Tests

```
go test ./...
pnpm --dir frontend test
```

Unit tests cover decode, clocks, fill zoom, scheduling, title-card paint, and the 20-second two-clip timing case. One test resolves a document through a fake probe and checks that both exports carry the compiler's argv. When ffmpeg is present, smoke tests render one frame of each backdrop, one short clip of each move, one dissolve, and one file whose pixel format and audio codec are checked with ffprobe. A separate test writes a PNG and a WAV, renders a short hold, and checks the output size and duration.

The command-line tests run `cli` with a fake probe and a stand-in ffmpeg script. They cover help, every usage error, exit codes, JSON errors, stdin and `-dir`, overrides, `-o` files, progress, `-clean`, a failed step, an interrupt, `serve`, and a plan that matches `/v1`. One more test renders a real video when ffmpeg is installed. The desktop tests open the real GTK dialogs when a display is available, and send an editor save through the GUI's `/v1` bridge. The frontend tests (Vitest with happy-dom) cover the document operations, including a timeline drop and a divider drag, the editor screen, and the GUI host.
