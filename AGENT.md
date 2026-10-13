# Agent notes

Read `montage-v1.md` before changing resolution, the document schema, or ffmpeg output. It is the normative spec. `IDEAS.md` is the design record. Where they disagree, the spec wins.

The application is kbvmaker. The format is Montage.

## Version boundary

The module is `neal/kbvmaker`. Montage 1 is `neal/kbvmaker/v1`.

`v1.Version`, the document field `montage`, and the export field `renderPlan` are `v1/version.Current`. Today that is 1. Reject any other `montage` value from this package.

A later Montage is a new package (`v2`) that accepts `montage: 2`. Keep `v1` able to load a version-1 document. Do not accept two document versions in one package. `doc` imports `v1/version`, not package `v1`, so the version constant does not cycle.

## Where logic lives

```
Document → Plan → []Command → bash | renderPlan JSON | exec
```

The CLI, `web/v1`, and the Wails window call `v1`. They do not contain resolution rules. HTTP lives in `web/v1`. The Svelte client lives in `frontend/src/lib/montage/v1`. A later Montage is `web/v2` and `frontend/src/lib/montage/v2`. Shared UI is `frontend/src/lib/components/ui` and `frontend/src/lib/platform`.

The CLI in `cmd/kbvmaker` imports the `v1` facade, `v1/doc` and `v1/model` types, and `web` for `serve`. It does not call `plan`, `compile`, or `run` directly. Its contract is in `CLI.md`: results on stdout or `-o`, progress and errors on stderr, exit codes 0/1/2/3/4/5/130, and `-json` errors shaped like `/v1`. Changing an exit code, a stream, or the JSON shape breaks pipelines. Update `CLI.md` and `cmd/kbvmaker/cli_test.go` with it. The CLI does not create or edit documents yet. That waits on moving the editor's document operations from `frontend/src/lib/montage/v1/document.ts` into Go.

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

## Building the GUI

`./scripts/build` builds the client and the GUI. On Linux it selects WebKitGTK 4.1 or 4.0 with pkg-config. Windows PowerShell uses `scripts/build.ps1`. Leave those commands out of the README. The script prints the package to install when the library is missing.

## Tests

`go test ./...` is the Go check. Add a test at the seam you changed: decode, clock, frame, schedule, timing, motion, check, golden argv, or an HTTP handler. `pnpm test` in `frontend/` is the client check. Add a test for a v1 client behavior or a screen you changed.

The render test in `v1/render_test.go` writes a PNG and a short WAV, renders a hold, and probes the output. Run it when a change can affect ffmpeg filters, size, or duration. It skips when ffmpeg is absent.

## Out of scope unless asked

Do not commit, push, or open a pull request. Do not add session management or a second engine binding for Wails. The window's only non-dialog binding is `API.Call`, which carries `/v1` requests to the same handler because older WebKitGTK drops POST bodies. Baking a generated schedule back into a Montage file is not a user-facing function yet. Creative rotation and skew stay out of version 1.
