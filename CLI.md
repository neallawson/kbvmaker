# kbvmaker command line

`kbvmaker` checks, plans, exports, and renders a Montage document. It is meant
for scripts and pipelines. It calls the same `v1` engine functions as the HTTP
API at `/v1`, so a command-line plan and an API plan of the same document are
identical, and a test checks that.

```
go build -o kbvmaker ./cmd/kbvmaker
kbvmaker help
kbvmaker help render
```

## Commands

| Command | Does | Writes |
| --- | --- | --- |
| `validate FILE` | Checks the document against the Montage rules. Reads no media. | `ok` |
| `validate --media FILE` | Also probes the song, every picture, and the ffmpeg filters. | `ok: 24 pictures, 3:42` |
| `plan FILE` | Resolves the timeline. | Plan JSON |
| `export-script FILE` | Compiles the render as bash. | Script |
| `export-plan FILE` | Compiles the render as `renderPlan` JSON. | JSON |
| `render FILE` | Runs ffmpeg and writes the video. | The video's path |
| `serve` | Serves `/v1` (and the client with `-static DIR`) until interrupted. | — |
| `version` | Build and Montage version. | `kbvmaker 9d847d0… (Montage 1)` |

`FILE` is a Montage document. `-` reads it from stdin; relative paths inside it
then resolve against `-dir` (default: the current directory). `-dir` with a
named file is a usage error, because the file's own directory is the project
directory.

Flags may come before or after `FILE`. Everything after `--` is a file name,
even if it starts with `-`.

## Streams

- **stdout** carries the result: plan JSON, the script, the `renderPlan`, the
  rendered video's path, or `ok`. Nothing else is printed there.
- **stderr** carries progress, the `listening` line from `serve`, and errors.
- `-o FILE` (or `-output FILE`) sends the result to a file instead.
  - `plan`, `export-script`, and `export-plan` write it to a temporary file
    in the same directory and rename it. A reader never sees half a file. A
    script is written with mode 0755.
  - On `render`, `-o` is the video path and overrides `output.file`.

## Errors and exit codes

| Code | Meaning | Error `code` |
| --- | --- | --- |
| 0 | Success | |
| 1 | Unexpected failure (I/O and the like) | `error` |
| 2 | Usage: unknown command, bad flag value, wrong number of files | `usage` |
| 3 | The document breaks a Montage rule | the rule code, for example `bad-number`, `mode`, `bad-time` |
| 4 | Missing media or tool: document, song, picture, backdrop image, ffmpeg filter, or ffmpeg/ffprobe itself | `missing-media`, `missing-tool` |
| 5 | An ffmpeg or ffprobe step failed during render | `render` |
| 130 | Interrupted by SIGINT or SIGTERM | `interrupted` |

`-json` prints an error as one line on stderr, in the same shape as the HTTP
API:

```json
{"slot":2,"code":"missing-media","message":"image b.jpg"}
```

`slot` is 1-based, and 0 means the whole document. A `render` error message
names the stage and includes ffmpeg's output.

With `-json`, results that are not files are JSON too: `validate` prints
`{"ok":true}` (with `slots` and `duration` under `--media`), `render` prints
`{"file":…,"seconds":…}`, and `version` prints
`{"version":…,"montage":1,"renderPlan":1}`.

## Overrides

The document is the record and flags win for one run. Nothing is written back
to the file.

| Flag | Overrides |
| --- | --- |
| `-audio FILE` | `audio` |
| `-o FILE` (render) | `output.file` |
| `-join SECONDS` | `defaults.join.duration` |
| `-hold SECONDS` | `defaults.hold` |
| `-zoom-span N` | `defaults.zoomSpan` |
| `-blur N` | the blur of a `blur` or `image` canvas backdrop |
| `-seed N` | `schedule.seed` (a pool without a schedule becomes `shuffle`) |
| `-schedule POLICY` | `schedule.policy` |

Paths given on the command line are relative to the current directory, not
to the document. A value that breaks a rule fails like a bad document: exit 3,
naming the rule.

## Rendering

```
kbvmaker render piece.montage.json
```

- Progress goes to stderr, one line per step:

  ```
  [5/30] clip 3/24
  ...
  wrote /films/piece/out.mp4 in 4:12
  ```

  `-progress json` prints one JSON object per line instead:

  ```json
  {"event":"start","stage":"clip","step":5,"steps":30,"n":3,"of":24}
  {"event":"done","stage":"clip","step":5,"steps":30,"n":3,"of":24,"seconds":7.9}
  {"event":"wrote","file":"/films/piece/out.mp4","seconds":252.1}
  ```

  `-progress none` prints nothing until it finishes.
- The video's absolute path goes to stdout when it is done, so
  `video=$(kbvmaker render piece.montage.json)` works.
- The last step writes to a hidden `.NAME.partial.EXT` next to the output and
  renames it at the end. A failed or interrupted render leaves no video at the
  output path. An earlier video there is replaced only on success.
- SIGINT or SIGTERM stops the running ffmpeg and exits 130.
- Intermediate canvases, clips, and joins go to `work/` under the project
  directory. `-work DIR` moves them (relative to the project directory unless
  absolute). `-clean` deletes them after a successful render. It removes only
  the files this render named (`canvas-N.png`, `clip-NNN.mp4`,
  `join-NNN.mp4`), then the directory if it is empty and is not the project
  directory. A failed render keeps them for inspection.
- ffmpeg prints only errors and its progress line. `-verbose` keeps its banner
  and warnings. That shows in exported scripts and in a failed step's message.
- `-ffmpeg BINARY` and `-ffprobe BINARY` choose the tools. The name given is
  also what an exported script calls.

## Pipelines

```sh
# Check every project, stop on the first bad one.
for f in projects/*.montage.json; do
  kbvmaker validate --media --json "$f" || exit
done

# Render with a different song, keep the path, report machine-readable progress.
video=$(kbvmaker render --audio take2.wav --progress json --clean piece.montage.json 2>progress.jsonl)

# A generated document on stdin, rendered against a media directory.
generate-montage | kbvmaker render --dir /media/greenhouse -o /out/greenhouse.mp4 -

# Branch on the kind of failure.
kbvmaker render piece.montage.json
case $? in
  0) echo done ;;
  3) echo "fix the document" ;;
  4) echo "a photo, the song, or ffmpeg is missing" ;;
  5) echo "ffmpeg failed" ;;
esac
```

## Not yet

The command line reads and renders Montage files. It does not create or edit
them yet. Creating a project, adding pictures, setting the song, pinning a
length, applying a look, and shuffling still happen in the window. The plan is
to move those edits into a Go package under `v1`, so the window and the
command line share them through the engine.
