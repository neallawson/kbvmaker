# Agent notes

Read `montage-v1.md` before changing resolution, the document schema, or ffmpeg output. It is the normative spec. `IDEAS.md` is the design record. Where they disagree, the spec wins.

The application is kbvmaker. The format is Montage. `make_Greenhouse_video_v3.sh` is a reference tape, not code to extend.

## Version boundary

The module is `neal/kbvmaker`. Montage 1 is `neal/kbvmaker/v1`.

`v1.Version`, the document field `montage`, and the export field `renderPlan` are `v1/version.Current`. Today that is 1. Reject any other `montage` value from this package.

A later Montage is a new package (`v2`) that accepts `montage: 2`. Keep `v1` able to load a version-1 document. Do not accept two document versions in one package. `doc` imports `v1/version`, not package `v1`, so the version constant does not cycle.

## Where logic lives

```
Document → Plan → []Command → bash | renderPlan JSON | exec
```

The CLI and a future Wails window call `v1`. They do not contain resolution rules.

Probing media and executing ffmpeg are the only edges. Packages between them take and return values.

| Change | Where it goes |
| --- | --- |
| New timing policy, join type, or easing | A row in the existing table. |
| New built-in mode | A catalog entry in `v1/motion`. |
| New export | Another function of `[]Command`. |
| New check | `v1/check`, with a stable code from `v1/rule`. |

There is no plugin loader. Shared resolved types live in `v1/model` so checks do not import the orchestrator.

## Document rules

Save the edit the user authored. Recompute durations, fade offsets, fill zoom, and `zoompan` expressions. Do not write those derived numbers back into the Montage file.

Unknown JSON keys are errors. Duplicate keys are errors. `encoding/json` does not reject duplicate keys; the walker in `v1/doc` does.

Zoom `1.0` means the whole photograph is in frame. Apply rotation metadata before fill zoom. Keep ffmpeg stderr. Paths are argv elements, not shell-parsed strings.

## Tests

`go test ./...` is the check. Add a test at the seam you changed: decode, clock, frame, schedule, timing, motion, check, or golden argv.

The render test in `v1/render_test.go` writes a PNG and a short WAV, renders a hold, and probes the output. Run it when a change can affect ffmpeg filters, size, or duration. It skips when ffmpeg is absent.

## Out of scope unless asked

Do not start the GUI. Do not commit, push, or open a pull request. Flag overrides exist as `doc.ApplyOverrides` and are not wired to the CLI yet. Baking a generated schedule back into a Montage file is not a user-facing function yet.
