# Making the Ken Burns video tool generic

Notes from the Greenhouse tape work, so the next session can start from the
design instead of the chat. The album names, the fixed list of 13 photos, and
`i % 4` pattern selection were specific to that one video.

## Invariants already settled

These came out of a real failure. Keep them.

Zoom `1.0` means the entire photograph is in the frame. The old pipeline
scaled with `force_original_aspect_ratio=increase` and cropped to 16:9
*before* `zoompan`, so zoom `1.0` was only the center crop. Every photo in
this set is taller than 16:9. Portraits lost about 60% of their height before
the move started, and zooming back out could not bring it back. Lowering the
cap from `1.4` to `1.17` does not help: the discarded pixels are already gone.

The working canvas is 3840×2160. A blurred, cover-cropped copy of the same
photo fills the bars (`gblur`, `BLUR_SIGMA`). The sharp copy is fitted with
`force_original_aspect_ratio=decrease` and centered. `setsar=1` on both
branches, because rounding the fit onto 3840×2160 can leave a non-square
sample aspect ratio. The blur runs once into `canvas_N.png` (`-update 1`);
`zoompan` then reads that still. Zoompan output is 1920×1080.

`zoompan`'s `time` variable is the output timestamp in seconds, and it does
reach the end of the clip. A zoom-out expression that is paired with the clip
length really does land on zoom `1.0`. The bug was the crop, not the clock.

Speed and range stay coupled. `ZOOM_SPAN` (0.4) spread across a whole clip
sets `ZOOM_SPEED` (about 0.017 zoom units per second on this song). The move
itself only runs outside the crossfade and a hold, so the far end is

```
ZOOM_LIMIT = FULL_ZOOM + ZOOM_SPEED * TRAVEL_DUR
TRAVEL_DUR = CLIP_DUR - XFADE_DUR - HOLD_DUR
```

Per photo, the far zoom is the smaller of `ZOOM_LIMIT` and the zoom that just
fills the 16:9 frame (`FILL_ZOOM`). Near-16:9 shots fill the screen at the
tight end. Portraits would need roughly 2.4–2.7 to fill the width, which this
speed cannot cover, so they get a mild punch-in and the bars stay visible at
the tight end. The other end is the whole photo.

The whole photo has to be on screen while the image is fully opaque, not only
during the dissolve:

- Zoom-in patterns hold zoom `1.0` through the fade-in plus `HOLD_DUR` (1s), then move.
- The zoom-out pattern arrives at zoom `1.0` one second before the fade-out and stays there.

`TRAVEL_DUR` must stay positive. A clip shorter than the crossfade plus the
hold has no room to move. The script already exits when that happens.

Clip length for N equal clips crossfaded to a target duration:

```
CLIP_DUR = (TARGET_DUR + (N - 1) * XFADE_DUR) / N
total    = N * CLIP_DUR - (N - 1) * XFADE_DUR
```

Any count of pictures already works, as long as each clip stays long enough.

## What is already generic

- Timing from N, the crossfade, and the target duration.
- Per-photo fill zoom from aspect ratio.
- The canvas filter. It does not care which files the photos are.
- The crossfade chain, built in a loop for N−1 joins.
- The rule that one end of every move is the whole photo.

## What is still this album

- `AUDIO`, `OUTPUT`, and `TARGET_DUR=282` written at the top.
- The `IMAGES` array, in an order that does not match a filename sort.
- Pattern choice via `i % 4`.
- Canvases rebuilt for every slot, even if a later scheduler repeats a file.
- Temp files (`clip_*.mp4`, `canvas_*.png`) in the working directory, removed with a glob.
- ffmpeg's log swallowed by `grep | tail`, so a failure is a last line rather than the error.
- The first and last clips still hold through a fade they do not have. Harmless, and worth skipping once clip timing is special-cased.

These photos had no rotation metadata. A generic tool should still honor
ffmpeg's display orientation (autorotate, or swap width and height on a 90°
or 270° display matrix) before computing `FILL_ZOOM`. ffprobe's coded
width/height is pre-rotation.

## Scheduling

The current timeline *is* the picture list, and clips get shorter as pictures
are added. Looping and shuffling want the opposite.

Pick how long a picture stays up. Read the song length with `ffprobe` instead
of a hardcoded duration. Fill that many slots from a pool. For this song, a
10s clip and a 1.5s crossfade is about 33 slots. The existing one-shot
crossfade can join that many.

Slot policies to support, as a schedule name:

- Given order (the file, or the array, as written).
- Shuffle once.
- Loop / random draw, with no immediate repeat of the same photo.

Pattern choice belongs on the slot, so the same photo can use a different
move the next time it appears. Cache the fitted canvas by source path.

The formula to invert, when clip length is the input and slot count falls out:

```
total = N * (CLIP_DUR - XFADE_DUR) + XFADE_DUR
```

## Modes

The four patterns are two zoom directions and three pans: center,
left-to-right, top-to-bottom. A one-move mode is a direction plus a pan path.
Reverses and diagonals are more rows in that table, not a new renderer.

Every one-move mode keeps the settled rule: one end is zoom `1.0`, and that
end lands inside the fully opaque part of the clip. Pan progress for zoom-in
moves is 0 during the opening hold and 1 at the end of the clip, multiplied
by `(iw - iw/zoom)` or `(ih - ih/zoom)`, so the frame stays put while the
whole photo is showing.

A move with two phases is where a `case` statement stops being enough. Zoom
in, hold, then pan across the overflow — or zoom out and then pan — is a list
of segments per clip, still rendered by the same `zoompan` pass. That is the
point at which a mode is data rather than another branch.

Modes that show the whole photo in one frame stay inside this model. A mode
that only *scans* a portrait (zoomed to fill, panning the overflow, never
pulling back to zoom `1.0`) would give up the invariant on purpose. Treat
that as a different kind of mode, not as a default.

## Joining clips

One `filter_complex` of crossfades is fine into the tens of inputs. A much
longer piece, or several songs sharing a temp directory, wants that join done
in batches. The concat demuxer does not replace `xfade`; the dissolves still
have to be filter graphs, just not one graph for the whole song.

Argument lists and filter graphs also hit command-line length limits before
the zoom math does. Batching is the fix, when a run actually gets that long.

## How to specify a run

Do not keep the recipe in the engine script.

**Flags** for values that change every run and are single values: audio path,
output path, crossfade, hold, zoom span, blur, random seed, and the schedule
name (given order, shuffle, loop). Flags override the file, so the file is
the saved recipe and the command is the experiment.

**Song length** comes from `ffprobe` on the audio file.

**Picture list** in a plain text file when the order is deliberate and will
be reused. One path per line. Blank lines and `#` comments ignored. A
directory of images is the other input, with a sort you choose. Ordinary
filename order sorts `Greenhouse10.jpg` before `Greenhouse2.jpg`, which is
why this album's list is written out by hand. Natural sort, or an explicit
file, avoids that.

When a slot is a photo plus a move, the same file grows a second column:

```
Greenhouse3.jpg  out-center
Greenhouse8.jpg  in-left-right
```

A line-oriented file is enough while a slot has only a couple of fields. JSON
or YAML starts to pay once a slot or a mode has more structure than that.
Either way, keep the mode catalog and the per-album timeline as data the
engine loads. The project file and the time points it can name are specified
below.

A short per-album shell script that only sets variables and calls the engine
is a workable command file and the smallest step from today. It gets worse
when that script grows its own logic and the photo list ends up back inside
the zoom math.

Environment variables are a poor place for the recipe. They disappear from
the command you re-run, and a list of paths in one variable is awkward to quote.

## Suggested shape when this moves to source

1. Engine with no album filenames in it.
2. Flags for the single-value options, defaulting from a project file when one is passed.
3. Image list from a file and/or a directory-plus-sort.
4. Duration from the audio file.
5. Schedule that turns a pool into slots (order, shuffle, loop) and assigns a one-move pattern per slot.
6. Canvas cache keyed by source path.
7. Leave two-phase moves and batched crossfades until a real timeline needs them.

Shell can carry steps 1–6. A separate program starts to earn its keep when
the mode list and the slot rules are what you edit, or when a clip is a
sequence of moves rather than one expression.

## Tool justification

The app is a composer. It plans a timeline, then calls ffmpeg. The motion
model lives in Go. The window edits a project file and shows the plan.

Go holds the engine: timing, fill zoom, schedule policies, and filter-graph
construction as types that can be tested without a window or an encode. A
project goes in and a list of ffmpeg invocations comes out. One binary covers
Linux, Windows, and Mac. ffmpeg stays an external program the engine probes
and execs.

Wails is the GUI. It gives native file dialogs and a real window on all three
platforms, with a web view for the form: pick audio and pictures, order them,
assign a move, see the timing before render. The same view can grow a
timeline later.

Svelte and Vite are the frontend. The screens are lists, selects, and a
progress log. Svelte is a first-class Wails frontend and stays small for that.

The engine comes first, with no album names and no window: project file,
flags, duration from ffprobe, schedule (given order, shuffle, loop), canvas
cache, one-move patterns as rows of data. A small CLI renders a project and
proves the package. The Wails app loads and saves the same project and calls
the same package. The GUI picks files and shows the plan. Zoom math and
zoompan expressions stay in Go.

A slot is a source plus a mode id now, and a source plus an ordered list of
segments later. Two-phase moves and batched crossfades stay unimplemented
until a timeline needs them. The types already allow a mode to be data.

ffmpeg is found on PATH or in a configured location. The app checks that the
build can do zoompan, xfade, and gblur, and surfaces the real ffmpeg error.
Packaging a private ffmpeg is a later distribution choice.

On Linux, Wails needs WebKitGTK. That is the packaging cost. Windows
WebView2 and macOS WKWebView are quieter. A release targets Ubuntu 22.04 or
newer with `libwebkit2gtk-4.1`, built with the `webkit2_41` tag. The older
4.0 build keeps working for development but is not promised.

The GUI earns its keep with a plan preview: slot count, clip length,
per-photo fill zoom, and which end of the move is the whole photo, including
portraits where the bars stay visible at the tight end. Rendering one still
at the start, the hold, and the far zoom is enough to check a pattern. A
live zoompan preview can wait.

## Project file

The file stores the edit. The engine resolves it into timings and ffmpeg.
The format is named Montage. It is a composition format: a closed JSON
vocabulary plus a resolution procedure, specified normatively in
`montage-v1.md`. These notes are the design record. Where they disagree
with that specification, the specification wins.

A versioned JSON project is the language for that: small, typed, and open,
with room to grow into multi-segment moves while staying data. It does not
become a scripting language. Easing stays an enum. A segment does not grow
an expression string.

The document is `montage: 1`. Paths are relative to the file. Media stays on
disk, so the file stays small and diffable. Unknown fields are rejected in
version 1, so a typo fails the load instead of being ignored. JSON is the
canonical form because the GUI reads and writes it and Go can check it
against a schema. A line-oriented photo list is how a pool gets imported.
The import becomes slots. That list is not the format to grow.

The engine keeps three representations, and only the first is saved.

1. The project file. Assets, defaults, and the operations the user chose.
2. The resolved timeline. Clip lengths, song-time offsets, per-photo fill
   zoom, the far zoom after clamping, and where zoom `1.0` sits inside the
   opaque part of each clip. A plan command prints it, and the GUI previews
   it. It is recomputed when the file or the media changes, and it is not
   written back. Stored derived numbers would lie on the next edit.
3. The renderer. Canvas filters, `zoompan` expressions, and the `xfade`
   graph. Those are an output of the engine, built at render time the way
   the shell script builds them today. ffmpeg expressions are not stored in
   the file.

A slot on disk is a mode name or a list of segments, one of the two. Both
expand to the same segment list before render. The four patterns in the
shell script are the first four catalog entries. A project may add its own
modes. The GUI writes the mode until the user detaches that slot, then it
writes the segments and clears the mode. `basedOn` may keep the catalog name
the segments started from, so the GUI can still show it.

The script mixes the next four concerns into one loop. The format keeps
them apart, because they grow on different schedules.

**Framing** is a property of the source: working canvas, blur, and the fit
that makes zoom `1.0` the whole photograph. Cached by path. Repeated use of
one photo reuses one canvas.

**The join** is the transition between two slots: a type and a duration.
Version 1 has one type, `fade`. Later transition names are more values of
that type. The first slot has no fade in and the last has no fade out.
Opaque time is derived from the joins a slot actually has, which drops the
hold-through-a-missing-fade special case in the script.

**Motion** is zoom and pan inside one slot.

**Timing policy** chooses durations.

- `fit-audio` reads the song length with `ffprobe` and divides it into equal
  clips, using the formula already in the script.
- `fixed-clip` takes a clip length, the slot count falls out, and a pool
  fills those slots.
- `explicit` puts a duration on each slot.
- `markers` places cuts on the song clock. See Time points. Markers replace
  the even split when the cuts follow the song.

A pool plus a schedule (`given`, `shuffle`, `loop`, and a seed) is a
generator in front of the slot list. Baking the schedule writes the concrete
slots into the file and drops the seed, so a later render does not reshuffle.

Zoom endpoints in the file are a number, or a symbol the engine computes:

- `full` is zoom `1.0`, the whole photo.
- `fill` is the per-photo zoom that just covers 16:9.
- `limit` is as far as `zoomSpan` can travel in this clip's travel time.

The default far end is the smaller of `fill` and `limit` (`MAX_ZOOM` in the
script). Speed is derived from the endpoints and the resolved duration.
`zoomSpan` is a project default, not a per-segment speed. Storing a speed
such as the script's 0.017 zoom units per second would freeze a derived
value. Endpoints plus `zoomSpan` let speed fall out of the clip length.

Pan is a subject point on the photograph, `u` and `v` in `0..1`: this point
of the photo wants the center of the frame. Named points `center`, `left`,
`right`, `top`, `bottom`, and the corners mean as far as the current zoom
allows, and the engine clamps them. At zoom `1.0` every point clamps to
center, so the frame stays put while the whole photo is showing. Pixel `x`
and `y` on the working canvas are a compile target. They change if the
working size changes, and they change as zoom changes the overflow.

A segment is one span. A hold is a span whose ends match. A one-move pattern
is two spans: hold the whole photo, then travel. A two-phase move is three
spans on the same list, rendered by the same `zoompan` pass. Easing is
`linear` in version 1. `smoothstep` is a later value.

```json
{
  "from": { "anchor": "opaque-start" },
  "dur": 1.0,
  "zoom": "full",
  "pan": "center"
}
```

```json
{
  "dur": "rest",
  "zoom": { "from": "full", "to": { "min": ["fill", "limit"] } },
  "pan": { "from": "center", "to": "right" },
  "ease": "linear"
}
```

Each catalog entry has a family. `reveal` must put zoom `full` on screen
during the opaque interval, and the engine rejects a reveal that never does.
`scan` opts out of that rule on purpose: the photo stays at `fill` and only
pans, which is the portrait scan that never pulls back to zoom `1.0`. The
family mark is that opt-out, so it cannot become the default.

Mode names are the normal authoring surface. Inline segments are the escape
hatch. Encode settings (codec, crf, preset) live in `output` beside the
frame size. They are part of the project and outside the motion language.

```json
{
  "montage": 1,
  "canvas": { "width": 3840, "height": 2160, "blur": 40 },
  "output": { "width": 1920, "height": 1080, "fps": 30 },
  "defaults": {
    "join": { "type": "fade", "duration": 1.5 },
    "hold": 1.0,
    "zoomSpan": 0.4
  },
  "tracks": [{"src": "Love_Ruins-Greenhouse.mp3"}],
  "timing": { "policy": "fit-audio" },
  "slots": [
    { "src": "Greenhouse3.jpg", "mode": "out-center" },
    { "src": "Greenhouse8.jpg", "mode": "in-left-right" }
  ]
}
```

The format may express a bad edit. Before render, the engine refuses it and
names the slot and the rule:

- A moving mode has positive travel time.
- Segments cover the clip. A gap holds the previous zoom and subject.
- Pan endpoints clamp inside the photo at that zoom.
- A `reveal` motion reaches zoom `full` while fully opaque.
- Each join is shorter than either clip it overlaps, and under `markers`
  each gap between cuts is longer than that boundary's join, so every
  picture has a fully opaque instant.

Version 1 of the engine expands the current four modes, `fit-audio`, and
`fade`. The schema already has a segment list, symbolic zoom endpoints,
anchors, and a `scan` family. Two-phase moves and new joins are new catalog
rows and a checker, not a new file format.

## Time points

Percent-of-song and seconds are two ways to write a time. The file supports
both, plus the other forms in this section. Each form stays still under a
different edit. Every form resolves to seconds on one timeline before any
`zoompan` expression is built. `zoompan`'s `time` variable stays in seconds
of the rendered clip. The file never speaks that variable.

A time value names a clock, and then either a position on it or a length.

**Clocks**

- `song` — the audio duration from `ffprobe`.
- `clip` — one slot, including its fades.
- `opaque` — the part of that clip where the picture is fully visible.

The first slot's opaque start is the clip start. The last slot's opaque end
is the clip end. A middle slot's opaque range sits strictly inside the clip,
after the fade-in and before the fade-out.

Travel is not a clock. Travel is the opaque time left after holds, and a
hold is often the endpoint being written, so a percent of travel can depend
on itself. The length `rest` is the spelling for whatever remains.

**Seconds are an absolute position.** `{ "sec": 92.4, "of": "song" }` is
92.4 seconds from the start of the audio. `{ "sec": 3, "of": "clip" }` is
three seconds after this clip starts. `{ "sec": 0, "of": "opaque" }` is the
instant the picture becomes fully visible. A negative second counts back
from the end of that clock: `{ "sec": -2, "of": "song" }` is two seconds
before the audio ends.

A timecode `HH:MM:SS.mmm` and a frame number at the project fps are further
spellings of an absolute song position. The frame spelling converts as
`frame / fps`. They exist so a marker copied from a DAW can be pasted
without a hand conversion. They resolve through the same path as `sec`.

**Percent is a position relative to a length.** `{ "pct": 41, "of": "song" }`
is `0.41` times the measured song duration. `{ "pct": 50, "of": "clip" }` is
halfway through that clip, fades included. `{ "pct": 50, "of": "opaque" }`
is halfway through the fully visible stretch.

Percent and seconds-from-the-start name the same kind of point, and they
diverge when the clock's length changes. A percent tracks a new duration of
the same arrangement, which is what a remaster or a longer export wants. A
raw second stays on the original spot, and lands past the end if the audio
gets shorter. Write a percent for "at the chorus." Write seconds when the
hit is a known timecode.

**An anchor plus an offset is relative to an event.** The anchors are
`song-start`, `song-end`, `clip-start`, `clip-end`, `opaque-start`, and
`opaque-end`. `{ "anchor": "opaque-start", "offset": 1.0 }` is the script's
hold after the fade-in. `{ "anchor": "opaque-end", "offset": -1.0 }` is the
zoom-out arriving one second before the fade-out. This form stays put when
the song length changes and when a neighboring cut moves. It moves when this
slot's join changes. "One second after the photo is fully up" refers to the
fade, and neither a song percent nor a song second says that. Move a marker
and the anchor still means one second after the photo is fully up. A song
percent inside that slot moves with the cut.

**A length is not a position.** `{ "dur": 1.0 }` on a segment is one second
measured from the end of the previous segment, or from the start of the clip
when the segment is first. `{ "dur": "rest" }` runs to `clip-end`. A one-move
pattern uses `rest` for the travel span so the segment does not have to name
the clip length.

A song percent is also legal inside a segment, once the slot windows exist,
and only when that range lies inside the slot:

```json
{
  "from": { "pct": 41, "of": "song" },
  "to": { "pct": 55, "of": "song" },
  "zoom": { "from": "full", "to": { "min": ["fill", "limit"] } },
  "pan": "center"
}
```

**Markers** use any song-time spelling to place cuts. `until` is the handoff
from this picture to the next, and it is the middle of the dissolve, because
that is the beat you hear. The last slot has no `until` and runs to the end
of the song. Neighboring slots may mix percent, seconds, and timecode.

```json
"timing": { "policy": "markers" },
"slots": [
  {
    "src": "intro.jpg",
    "mode": "in-center",
    "until": { "pct": 18, "of": "song" }
  },
  {
    "src": "verse.jpg",
    "mode": "out-center",
    "until": { "sec": 92.4, "of": "song" }
  },
  { "src": "chorus.jpg", "mode": "in-left-right" }
]
```

Clip extents, with `x` the duration of that boundary's join. Cut points are
`C0 = 0`, then each `until`, then `C_N` at the song length.

- The first clip runs from `0` to `C1 + x/2`.
- A middle clip runs from `C_i - x/2` to `C_{i+1} + x/2`.
- The last clip runs from `C_{N-1} - x/2` to the end of the song.

The overlap of neighbors is exactly `x`, centered on the cut. A per-boundary
join duration, when a slot overrides the default, is the `x` for that cut.
Opaque time is the clip with the joins trimmed off: a middle picture is
fully visible from `C_i + x/2` to `C_{i+1} - x/2`.

**Resolution order**

1. Song duration comes from the audio.
2. The timing policy resolves slot windows. Marker percents and marker
   seconds both become cut points, then clip lengths, by the extents above.
3. Joins resolve each slot's opaque range.
4. Segment endpoints resolve in order. A point may be a position on `song`,
   `clip`, or `opaque`, or an anchor plus an offset. A `dur` adds to the
   previous endpoint. `rest` ends at `clip-end`.
5. A point that falls outside its slot is an error, and the message names
   the slot and the point. A song-time point inside a segment is legal when
   that instant lies inside the slot.

**Short slots.** Uneven markers produce uneven clips. A cut at 40% and the
next at 43% can be shorter than the crossfade plus the hold. If a gap
between cuts is not longer than the join, the picture is never fully opaque
and the dissolves collide. The engine rejects that pair of cut points. If
the opaque stretch is not longer than the holds, a moving mode has no travel
time, and the engine rejects it the same way the script exits on
`TRAVEL_DUR <= 0`. The message names the slot and the two cut points. A hold
mode, with no zoom span, stays legal on a short hit.

**Left out until there is a source for them.** Bars and beats need a tempo
map, and the audio file does not carry one. A percent of travel is left out
with travel itself, as described above.

## Render commands

Go resolves the Montage document before ffmpeg renders. The probes below are
the exception: song length, and each photograph's displayed size, including
a 90° or 270° display matrix. The plan is checked, and only then does ffmpeg
run. Paths are arguments, so a space in a filename is not a shell problem.
Work files live in a temporary directory. One canvas PNG is cached per
source path. The engine keeps ffmpeg's log.

For a two-picture, 20-second song under `fit-audio`, with the 1.5-second
fade and the 1.0-second hold, each clip is 10.75 seconds and the dissolve
starts at 9.25 seconds. The commands below use that plan.

The build must provide `zoompan`, `xfade`, and `gblur`:

```
ffmpeg -hide_banner -filters
ffprobe -v error -show_entries format=duration -of json song.mp3
ffprobe -v error -select_streams v:0 \
  -show_entries stream=width,height:stream_side_data=rotation:stream_tags=rotate \
  -of json Greenhouse3.jpg
```

Go expands each mode into segments and compiles a `z`, `x`, and `y`
expression per slot. That compilation does not call ffmpeg. The Montage file
never contains those strings. `zoompan`'s `time` is seconds from the start
of that clip.

One canvas still per distinct photograph. The blur runs once. Zoom `1` is
this picture, bars included. A repeated source reuses the PNG.

```
ffmpeg -y -i Greenhouse3.jpg \
  -filter_complex "[0:v]split[sharp][blur];\
[blur]scale=3840:2160:force_original_aspect_ratio=increase:flags=lanczos,crop=3840:2160,gblur=sigma=40,setsar=1[blurred];\
[sharp]scale=3840:2160:force_original_aspect_ratio=decrease:flags=lanczos,setsar=1[fitted];\
[blurred][fitted]overlay=(main_w-overlay_w)/2:(main_h-overlay_h)/2" \
  -frames:v 1 -update 1 "$WORK/canvas-0.png"
```

One clip per slot. `out-center` on the first slot of the example travels
until 8.25 seconds and then holds zoom `1`. With `zoomSpan` 0.4 the far zoom
compiles to about 1.307 when `fill` for that portrait is farther than the
clip can reach.

```
ffmpeg -y -loop 1 -framerate 30 -i "$WORK/canvas-0.png" \
  -vf "zoompan=z='if(lte(time,8.25),1.3070-(0.3070*time/8.25),1)':\
x='iw/2-(iw/zoom/2)':\
y='ih/2-(ih/zoom/2)':\
d=1:s=1920x1080:fps=30,format=yuv420p" \
  -t 10.75 -c:v libx264 -preset medium -crf 20 \
  "$WORK/clip-000.mp4"
```

`in-left-right` is the same command with a different filter. Zoom stays `1`
until the hold ends, then moves. Pan is the subject point on the photograph,
clamped into the window. At zoom `1` the clamp is the center, so the hold
does not slide. `x0`, `y0`, `dw`, and `dh` are the fitted rectangle measured
when the canvas was built.

```
z = if(lte(time,2.5), 1, 1+(0.3070*(time-2.5)/8.25))
u = if(lte(time,2.5), 0, (time-2.5)/8.25)
x = min(max(x0 + u*dw - iw/(2*zoom), 0), iw-iw/zoom)
y = min(max(y0 + 0.5*dh - ih/(2*zoom), 0), ih-ih/zoom)
```

One crossfade for a short timeline:

```
ffmpeg -y -i "$WORK/clip-000.mp4" -i "$WORK/clip-001.mp4" \
  -filter_complex "[0:v][1:v]xfade=transition=fade:duration=1.5:offset=9.25[v]" \
  -map "[v]" -c:v libx264 -preset medium -crf 20 -pix_fmt yuv420p -t 20 \
  "$WORK/joined.mp4"
```

`offset` is when that fade starts on the output timeline. A third picture
extends the same graph: the first join writes `[x1]`, and each later join
reads the previous label. Under `markers`, the offset is the cut minus half
the join, so the middle of the fade sits on the cut.

Mux the soundtrack. `-c:v copy` does not encode the picture again.
`-shortest` ends the file at the audio or the resolved video, whichever
finishes first.

```
ffmpeg -y -i "$WORK/joined.mp4" -i song.mp3 \
  -map 0:v:0 -map 1:a:0 \
  -c:v copy -c:a aac -b:a 192k \
  -shortest -movflags +faststart \
  piece.mp4
```

A timeline long enough to blow the argument list becomes several joins. Fade
a batch, then fade the batch results. The concat demuxer does not replace
`xfade`. Each batch is the same command as the crossfade above, with fewer
inputs.

## Script export

The resolved plan is a list of ffmpeg argument arrays. Writing that list as
a shell script is an export.

The Montage file stays the edit. The script is a snapshot taken after
resolution: durations, fade offsets, and `zoompan` expressions are already
numbers and strings. A shuffle is baked into a fixed slot order first, so
the script does not carry the random-number generator. Change the song or a
marker, and export again.

The wrapper is bash: `set -euo pipefail`, a work directory, quoted paths,
one canvas ffmpeg per distinct photograph, one clip ffmpeg per slot, the
`xfade` join, the audio mux, and a cleanup. ffmpeg's log stays in the
output. Someone with bash and ffmpeg can rebuild the video without
kbvmaker. That covers Linux and macOS, and Windows through a bash shell.
The ffmpeg arguments are the portable part. The bash file is one rendering
of the plan. A JSON list of the same argument arrays is the other.

## JSON export

The shell script and the JSON file are two writings of one resolved plan.
The JSON writing has no shell syntax. A runner on Linux, macOS, or Windows
execs each array directly, in order, and stops on the first non-zero status.
ffmpeg's log is the child's stderr. Paths are already separate arguments, so
nothing is quoted or split.

It is not a Montage document. Loading it back as a project is the wrong
operation. It is the snapshot described under Script export: durations, fade
offsets, and `zoompan` expressions are already numbers and strings, and a
shuffle has already been baked into a fixed slot order.

```json
{
  "renderPlan": 1,
  "commands": [
    ["ffmpeg", "-hide_banner", "-filters"],
    ["ffprobe", "-v", "error", "-show_entries", "format=duration",
      "-of", "json", "song.mp3"],
    ["ffmpeg", "-y", "-i", "Greenhouse3.jpg",
      "-filter_complex", "<canvas filter>",
      "-frames:v", "1", "-update", "1", "work/canvas-0.png"],
    ["ffmpeg", "-y", "-loop", "1", "-framerate", "30",
      "-i", "work/canvas-0.png",
      "-vf", "zoompan=z='...':x='...':y='...':d=1:s=1920x1080:fps=30,format=yuv420p",
      "-t", "10.75",
      "-c:v", "libx264", "-preset", "medium", "-crf", "20",
      "work/clip-000.mp4"]
  ]
}
```

Each command is the argv the engine would have passed to `exec`. The list
continues with the remaining clips, the `xfade` join or joins, and the audio
mux. Work paths are relative to the directory the runner uses for that plan.
Change the song or a marker, and export again.

## Engine

The Go module is `kbvmaker`. The Montage 1 API is the package tree
`v1`. `v1.Version`, the document field `montage`, and the export field
`renderPlan` are the same number. Montage 2 will be a new tree, `v2`, and
`v1` keeps loading version-1 documents. The CLI and, later, the Wails window
call `v1`. They do not contain the resolution rules.

The pipeline is pure between two edges. Probing media and executing ffmpeg
are the edges. Everything between them is a value.

```
Document → Plan → []Command → bash | renderPlan JSON | exec
```

A Document is the file after defaults and flag overrides. A Plan is the
resolved timeline: song length, video duration, and each slot's source,
mode, family, clip extent, opaque range, fill zoom, far zoom, and join
start and midpoint. A Command is one argv array plus a stage name (`filters`,
`probe`, `canvas`, `clip`, `join`, `mux`), or a `card` stage that writes a
PNG from bytes on the command. Bash and `renderPlan` are two formatters of
that list. Exec consumes the same list. `renderPlan` puts card PNGs in
`files` (path to base64) and keeps `commands` as ffmpeg argv.

Packages, in dependency order. Later ones import earlier ones.

| Package | Responsibility |
| --- | --- |
| `v1/rule` | Slot index, stable rule code, message. |
| `v1/doc` | JSON types, duplicate and unknown keys, defaults, structural checks. |
| `v1/model` | The Plan and the spans on a slot. No logic. |
| `v1/media` | `Probe`: song duration, displayed image size, ffmpeg filters. `ffprobe` is one implementation. |
| `v1/clock` | Seconds, percent, timecode, frames, and anchors, resolved on song, clip, and opaque clocks. |
| `v1/frame` | Fill zoom, the fitted rectangle, and the clamped window. |
| `v1/schedule` | Natural sort, splitmix64, shuffle, loop, pool listing. |
| `v1/timing` | `fit-audio`, `fixed-clip`, `explicit`, `markers`. |
| `v1/motion` | Built-in catalog, project modes, segment expansion, zoom `limit`. |
| `v1/check` | The rule table. `scan` skips `reveal` and still runs the rest. |
| `v1/plan` | The only orchestrator. |
| `v1/card` | Paint a title or credits card to an image. Liberation Sans and Serif are embedded. |
| `v1/compile` | Plan to commands, including an `xfade` batch split. A card slot is a PNG write plus a clip that reads it. |
| `v1/export` | Bash and `renderPlan` JSON. Card PNGs are base64 in the script and in `files`. |
| `v1/run` | Exec the argv list from the project directory, and write `card` PNG bytes. Stop on the first failure and keep stderr. |

A new timing policy, join type, or easing is a row in the table that already
exists. A new built-in mode is a catalog entry. A new export is another
function of `[]Command`. There is no plugin loader.

Tests follow the seams. Table tests cover decode, the 20-second two-clip
timing case, marker cuts, natural sort, the RNG, fill zoom, the clamp at
zoom `1`, each check code, and golden argv. One test loads a document
through a fake probe and checks that both exports carry the compiler's argv.
One test, skipped when ffmpeg is absent, writes a PNG and a WAV, renders a
short `hold`, and checks the output dimensions and duration with ffprobe.

## Front end

The window edits a Montage document and previews the plan. It does not keep
a second recipe. Load, tweak, and render all read and write `montage: 1`.
The engine still resolves time, zoom, and pan. The window shows the result.

Montage 1 is not locked. The repository is private and the app is still
changing. The specification describes what the engine does today. This
section is what version 1 is expected to include before anyone treats the
format as finished. New fields land in `montage-v1.md` when they are built.
Until then the spec wins for the code, and these notes win for what we are
willing to add.

Two ways through the same file.

The short path loads images, loads the soundtrack, leaves the suggested
look selected, and renders. That writes a complete document: slots in the
order the images were loaded, a plain move on each slot, a fill, a dissolve,
and a timing policy. A person who never opens a picture still gets a video.

The long path starts from that file. Reorder the picture list. Pick a
different look, a different fill, or a different move on one picture. Set a
length on one picture when the even split is wrong. Save a combination that
worked under a name. The file on disk is the edit, not a pointer back at the
preset that started it.

### Keeping it simple

Most people using this will not know what a zoom span, an opaque range, or a
reveal is, and a screen full of those controls will lose them. The short
path has to be: add the pictures, add the song, render.

Every choice they see is a preset with a plain name. The engine's names stay
in the file. "Slow zoom in" is what the window says. `in-center` is what the
document stores. A Look is one choice that sets the pacing, the moves, the
fill, and the join together, so the first screen is not four independent
menus.

The first screen offers a few Looks, one of them already selected. Further
Looks, the fill list, and the move list are one step away, still as names.
Enter and exit, a dragged point, a zoom number, and a blur amount are
another step down, on a single picture, after someone has decided the
suggested look is not enough. A control that only an author of the engine
would understand does not appear on the way to Render.

Presets are how the long path stays usable too. Customizing means starting
from a named setting and changing one thing, then saving that as a new name
if it should be reused. The window does not ask anyone to assemble a move
from zoom atoms on the first visit.

### What a slot edit is

A one-move slot has an enter state and an exit state. Each state is a zoom
and a point on the photograph. That is the commercial Ken Burns control,
the start rectangle and the end rectangle.

Zoom at an end is `full`, `fill`, `limit`, or a number at least `1`. The
point is one of the named places (`center`, `left`, `right`, `top`,
`bottom`, and the four corners) or a dragged `u,v` in `0..1`. Direction is
not its own field. Left-to-right is an enter point on the left and an exit
point on the right. The window can label that direction; the document stores
the two points. On the preset list those labels are the whole control.
`full`, `fill`, and `limit` show up only after someone opens More on that
picture.

`hold` is how long the enter state stays put before a zoom-in travels, or
how long the exit state rests before the outgoing join on a zoom-out. The
join on the slot is the transition that follows it. The short path never
sets a per-slot join. One transition preset covers the whole video.

While the slot still has a `mode` name, the window edits that mode. Every
other slot with the same name follows. When the user changes one picture
away from the shared move, the window detaches that slot: it writes
`motion`, clears `mode`, and sets `basedOn` to the name it left. The spec
already defines that handoff. The window says the picture is using its own
move, and still shows the name it started from.

A two-phase move (push in, then pan across the tight frame) is the same
editor with another pair of states in between. It is offered as a named
preset, not as a blank segment list.

Family stays `reveal` for anything the window builds from enter and exit,
unless the user picks a scan preset. A scan stays at `fill` and only pans.
The window names that preset ("Move across the photo"). It does not let an
untitled slot become a scan.

### Fill

The area the fitted photo does not cover is the canvas fill. The blurred
cover of the same photo is one fill, and it is the one the engine draws
today. It is not the one most people will want. Black bars, a flat color, a
quiet pattern, or a different backdrop picture will be ordinary choices, so
they are part of Montage 1, not a later version.

The document field is `backdrop`. `fill` stays the zoom that covers the
frame, so the bars do not reuse that word. A backdrop is a kind plus the
few values that kind needs:

```
{ "kind": "blur",    "blur": 40 }
{ "kind": "color",   "color": "#000000" }
{ "kind": "pattern", "name": "grain", "color": "#1a1a1a" }
{ "kind": "image",   "src": "backdrop.jpg", "blur": 0 }
```

`blur` is today's canvas: a cover-cropped copy of the same photo, with
`blur` as the sigma. An omitted `backdrop` means that, with sigma `40`.
`canvas.blur` still sets that sigma when `backdrop` is absent, so a document
that only names the old field means the same thing. Setting both is an
error.

`color` is a flat `#RRGGBB` in the bars. `image` is a different picture,
cover-cropped, with its own optional blur. The sigma defaults to `0` there,
so the backdrop picture is sharp until someone asks for blur.

`pattern` is a closed list. Version 1 has two:

- `grain` is uniform noise on the color (strength 20, seed 1).
- `gradient` runs from the color at the top to black at the bottom. It does
  not move. The gradient speed is 0, and the seed is 1.

An omitted pattern color is `#000000`. A pattern is not a formula and not a
user-drawn asset.

`canvas.backdrop` covers the video. `slot.backdrop` overrides it for one
picture, once that picture has been customized. The short path never offers
a per-picture backdrop. The window still calls this choice the fill.

The window's suggested Look writes an explicit black `color` fill, because
that is the bar treatment people already expect from a slideshow. "Blurred
photo" is a fill preset, and it is what the Greenhouse look uses. Other fill
presets are a handful of colors, a handful of patterns, and "choose a
backdrop picture." The sigma and the exact color sit under More. The preset
names do not.

Scale, as a separate slider, would mean a second definition of zoom. The
fitted photograph at zoom `1` is the whole picture. Enter zoom and exit zoom
are the scale. The window calls them zoom, and only inside More.

Rotation metadata is applied before fill zoom, and the window shows that as
a fact about the file. A creative rotation, and a skew, are out of version 1.
They are not fields, not presets, and not controls. A later Montage can add
them. This one does not.

### Joins

The document today has one join, `fade`, with a duration greater than zero.
That is the dissolve. The short path calls it Dissolve and uses one duration
for the whole video.

Slideshow joins may still be added to version 1. The `type` field was left
open for that. Nothing here freezes the set, and nothing here puts a
transition gallery on the first screen. A new join earns a place when a Look
or a transition preset would actually use it, under a plain name. The first
candidates, if we add any, are a cut and a dip to black. Wipes, page curls,
and 3D turns stay off the list until someone wants one of them enough to
name a preset. The default remains a dissolve either way.

### Two preset kinds

People see Looks, fills, moves, and transitions. Those are the presets. The
file still has only two stored shapes.

A **move preset** is a named mode. The four patterns in the shell script are
the first four, plus `hold`. The window's names for them:

| On screen | In the file |
| --- | --- |
| Slow zoom in | `in-center` |
| Pan right | `in-left-right` |
| Pan left | `in-right-left` |
| Drift down | `in-top-bottom` |
| Drift up | `in-bottom-top` |
| Slow zoom out | `out-center` |
| Hold still | `hold` |

The script's `i % 4` is not four presets of a video. It is one video that
cycles four moves. Photo film keeps three of them: slow zoom in, pan right,
and slow zoom out. The move menu on one picture lists every move. Shuffle
moves uses the Look's list, and Save writes that list on the Montage as
`moves`. Pan left, drift up, drift down, and hold still are in the menu.
The three shipped Looks do not assign them.

A **Look** is a composition preset: a recipe with no photographs in it. It
carries defaults (join, hold, zoom span, fill, ease), a timing policy, an
output size, and an assignment rule for modes. Applying it sets those
defaults and writes a `mode` on each slot. It does not replace the audio
path or the picture list.

Further move presets are more catalog rows, present so a Look can use them
and so More has a longer list. They are not the first screen. Reverses and
diagonals are the rest of the one-move table: `in-right-left`,
`in-bottom-top`, the `out-` pans, and the four corner diagonals. Each gets a
plain label when it is shown ("Pan back", "Drift up").

Scans are named presets too, family `scan`: move across, or move up and down
the photo, staying filled. A scan is how a wide group photo is treated when
pulling back to the whole frame would shrink the faces. A Look may assign
one. The home screen does not explain the family.

Two-phase presets are named the same way, once they exist: "Zoom in, then
pan" and "Pan, then zoom out." The segment list can hold them. The user
picks the name.

Shipped Looks, each a small document fragment. Three of them are enough for
the first screen. The rest sit under More looks.

- **Classic.** The suggested one. `fit-audio`, dissolve `1`, hold `0.5`,
  zoom span `0.25`, black bars, output `1920×1080`. Assignment alternates
  slow zoom in and slow zoom out. Load, render, done.
- **Photo film.** The shell script. `fit-audio`, fade `1.5`, hold `1`, zoom
  span `0.4`, blurred photo, same output size. Assignment cycles slow zoom
  in, pan right, and slow zoom out.
- **Quick slides.** `fixed-clip` of `5` seconds, dissolve `0.5`, hold `0`,
  zoom span `0.2`, black bars. Every picture is a slow zoom in. Pins are
  cleared, because this Look gives every picture the same length.

Further Looks, not on the first row:

- **Portraits and groups.** After the images are loaded, tall frames get a
  vertical move or a vertical scan, wide frames get a horizontal move. The
  window looks at aspect ratio once, at apply time, and writes a mode name
  per slot. The engine does not grow an aspect rule.
- **Open and close.** First and last pictures hold still. The middle uses
  the Classic alternation.

Assignment rules live in the window. The document receives mode names. The
rules are: one mode on every slot, cycle a list, alternate so adjacent slots
differ, or pick from aspect as above. Re-applying a Look is explicit. Slots
that have their own move keep it unless the user says to replace those too.

Fill presets and transition presets are the same idea at a smaller size.
Changing the fill does not reset the moves. Changing the transition does not
reset the fill. A Look sets all of them so the short path is one choice;
after that, each can be changed on its own.

### Library

Built-in moves ship in the engine catalog. Built-in Looks, fills, and
transitions ship as data the window lists.

A user's library is a directory of JSON fragments beside the app, not keys
inside the project. Saving asks for a name and stores the thing they
actually changed: a move, a fill, a transition, or a whole Look. Naming and
overwrite are the window's job. The list they see later is those names, next
to the built-in ones.

A project has to render on a machine that does not have that library.
Applying a user move copies the definition into the document's `modes`,
unless the name is a built-in. Applying a Look copies defaults and mode
names, and copies any user mode it references. After that, the Montage file
is the whole edit.

There is no Look field on the document. Remembering "this started as
Classic" and then changing the shipped preset underneath old files would
make a saved edit drift. The library holds the template. The file holds the
values.

### Picture length

The song is the clock when the project has one. `fit-audio` still makes the
video the length of the song. A picture with no `dur` shares that song with
the other unpinned pictures. The number the list shows for an unpinned
picture is the resolved clip length, and it is not written into the file.

Typing a length stores `dur` on that slot only. The other unpinned pictures
divide what remains, overlaps included. Clearing the box, or Remove custom
time, removes `dur` and puts the picture back in the share. A project with more than one picture
keeps at least one picture unpinned. A pin has to leave every remaining
picture longer than the dissolve. When it does not, the plan error names
that picture.

One picture is the whole song. Its length is shown and is not editable.

### No soundtrack

The clock is two tabs. **Soundtrack** is open for a new project: the song
picker and the song length. **No Soundtrack** is the picture clock. The
selected tab is the mode. Opening No Soundtrack removes the song and keeps
the picture lengths already on screen. Opening Soundtrack shows the picker
and does not change the clock until a file is chosen.

**Film length** is display only, the same kind of line as the song clock
(`m:ss · Ns`). **Time for each picture** is the one typed field. A new
project is `fixed-clip` of 5 seconds. Adding a picture adds that time.
Removing one gives it back. Ten pictures at 5 seconds with a 1 second
dissolve are a 41 second film: `N * (clip - x) + x`. The readout is that
sum.

A custom time stores `dur` on that slot and leaves the other pictures at
`clip`. The readout grows or shrinks with that change. After a grip drag
the film is `explicit` and has no single pace, so Time for each picture is
hidden and the per-picture lengths remain. Quick slides still clears those
times and keeps one length for every picture, so that field stays closed.

A song and a typed length are not both the clock. Adding a soundtrack while
Classic or Photo film is the Look hands the clock to the song and keeps
pins. Quick slides stays at its picture clock and the song is mixed to
whichever is shorter. An opened file that is already on a picture clock
keeps that clock when a song is added. Leaving the song stores `fixed-clip`
at the shared picture pace and keeps pins, so those lengths do not jump.
The editor does not write `timing.duration`. A file that already has it
still opens on No Soundtrack: the readout is that length, and editing Time
for each picture switches the file to `fixed-clip`.

`plan` and `render` accept the silent document, including an older file
that stores `timing.duration`. The mux writes the video stream and no audio
stream. Fit-audio with neither a song nor `duration` still cannot plan.
Markers and a pool still need a song.

The timeline draws these resolved lengths. Dragging a dissolve to change
them is under Timeline editing.

### Editor

One Montage file is the project. New starts from Classic in memory and does
not ask for a file. Save asks where to write `name.montage.json`. Open reads
a Montage that already exists. There is no second settings file.

The picture list is the editor. Adding pictures appends them in the order
they were picked. That order is the slot list, and it is what a later
rebuild uses. Sort by name, move up, and move down each write that list
once. A new file appearing in a folder does not reorder anything.

The timeline is labeled Timeline. Black rails on the top and bottom carry a
repeating row of sprocket holes, so the strip reads as film. The holes are
fixed to the strip. Each picture or card appears in the band between the rails as
soon as it is added, at an equal width. The block shows a thumbnail, and its number
when the block is wide enough. The same thumbnail sits beside the name in
the list. The file name, or the card's title, is the block's tooltip. Thumbnails are
`POST /v1/thumbnail`: the client names a path or a card and a pixel size, and the
reply is a base64 JPEG of exactly that size. A photograph is cover-cropped. A
card is painted to that box. When the song
is known, the blocks take their clip lengths, a darker band marks each
dissolve, and the axis runs from 0:00 to the film's length. Selecting a
row outlines that block. A press that does not travel selects that row.
Dragging a picture reorders it, and dragging the grip in a dissolve
changes the two lengths, as described under Timeline editing. The soundtrack's
length, in minutes:seconds and in seconds, sits beside the track list. A file
dialog opens again in the folder it used last.

The soundtrack is `tracks`, an ordered list of `{src, gap?}`. `gap` is
seconds of silence before that file. There is no `audio` field. The song
length is the sum of the file durations and those silences. `fit-audio`
divides the pictures across that length, so pictures keep playing through a
silence. Render concatenates the files and the silences into
`work/soundtrack.wav` when there is more than one file or any silence, then
muxes that one stream. One file and no silence is muxed directly. `-audio`
on one run replaces the list with that single file.

Under the picture strip, a soundtrack lane uses the same axis. Drag a file
to reorder it. The grip where a later file begins sets the silence before
it. Move soundtrack up, Move soundtrack down, Replace soundtrack, and Remove
soundtrack do the same work from the selected row. **Silence before** is the
typed form of the grip.

A long film keeps time-proportional widths. The lanes scroll together so a
picture stays wide enough to grab. **Film window** is a thin strip of the
whole film: picture ticks, soundtrack bars, and the open span. Dragging it
scrolls both lanes. The picture list is one line until the row is selected.
**Find a picture** jumps to a name. Thumbnails load for the open window and
the rows on screen.

The selected row sets the move by its plain name. The menu lists every
move. The length box is described above. Remove and replace change that
picture's file. Shuffle pictures reorders the slots that are not pinned in place. Sort by name writes names into those same open indexes. A pinned photograph and a pinned card both keep their index. Shuffle moves
assigns each picture one move from the Look's list. Both write the result
into the slots. Neither draws zoom numbers, and neither changes again on
the next open.

### Title and credit cards

A card is a slot. It has an order, a length, a dissolve, and a move, the
same way a photograph does. The Montage stores the text and the style, not
a PNG path. Save is still the only write into the project folder. At render
the engine paints a canvas-sized PNG (`work/card-N.png`) and feeds that
still to the existing canvas/clip/join/mux path. Thumbnails are the same
paint, JPEG, through `POST /v1/thumbnail` with `{card, width, height}`
instead of `{path, width, height}`. There is no `drawtext`, no ASS, and no
special ffmpeg.

The editor's **Add card** appends a title card: title "Title", Hold still,
center, middle, medium, bold, sans, white on black. The selected card shows
title, optional subtitle, optional third line, and a credits list (one
line per entry). Placement is align left/center/right and vertical
upper/middle/lower. Size is small/medium/large. Weight is regular/bold.
Face is sans or serif, from fonts shipped in the app (Liberation, SIL OFL).
Text color and background are `#RRGGBB`, shown as bordered swatches. The title,
subtitle, third line, and credits update the card when the field is left, so
the thumbnail is not painted on every key. An optional faint box sits behind the
text. Replace is off: there is no photograph to swap. Remove, move, length,
position pin, dissolve, and the move menu stay on. Drift up is the credits roll for a
tall list. Applying a Look, and Shuffle moves, leave a card on Hold still
so Classic does not zoom the letters. The user can still pick Drift up.
The timeline block uses the title (or the first credit line) and the
painted thumbnail.

Leave out of version 1: per-letter styling, arbitrary x/y, shadows,
outlines, a font file picker, and columns beyond a single credits line.

Looks are Classic, Photo film, and Quick slides. Classic is selected for a
new project. Applying a Look writes its defaults, a mode on every
picture, and that Look's move list. It does not rename the file to the Look.

- **Classic.** With a song, `fit-audio`. With no song, `fixed-clip` of 5
  seconds, and the finished length is shown. Dissolve `1`, hold `0.5`,
  zoom span `0.25`, black bars, output `1920×1080`. Modes alternate slow
  zoom in and slow zoom out.
- **Photo film.** The same clock as Classic. Dissolve `1.5`, hold `1`, zoom
  span `0.4`, blurred photo, same output size. Modes cycle slow zoom in, pan
  right, and slow zoom out. Shuffle moves draws from that list. The menu on
  one picture still lists every move.
- **Quick slides.** `fixed-clip` of `5` seconds, dissolve `0.5`, hold `0`,
  zoom span `0.2`, black bars. Every picture is slow zoom in. Pins are
  cleared, because this Look gives every picture the same length. A song
  does not restretch the pictures.

Save writes the Montage, including a project that does not yet have a song
or a picture. Export shell script and Render both save first. Each asks
where its file should go. Render and Export stay off until a plan can see
every picture and, when the film uses one, the soundtrack. That plan names
every missing soundtrack, picture, and backdrop above the buttons. **Re-scan Pictures/Audio** reads those files
from disk and loads the thumbnails again. The same check runs when the
window comes back to the front, so a file put back at its old path, or a
new picture saved over the same name, shows up without a change to the
Montage. Render shows the ffmpeg log when a command fails.
Unsaved edits are visible. **Save** is filled in while they are unsaved and cannot be clicked once they are written. Closing the window asks before discarding them. An engine error names the picture it belongs to.

Enter and exit stills, a per-picture fill, and a live ffmpeg stream are not
part of this screen yet. The length box and the move name are the picture
edit.

### Timeline editing

Two gestures. Each one changes how the pictures share
the film. The width of a block is that picture's time on screen, drawn to
scale. The list and the timeline keep one order and one set of lengths.
The length box stays the place a number is typed. Save is still the button
that writes the Montage to disk.

Drag a picture along the timeline and drop it. That writes the same slot
list as Move up and Move down. The picture keeps its file, its move, and
its pin, and it stays selected. A press that does not travel still selects
that row and opens a larger view of the whole picture. Click the view, or
press Escape, to close it. A drag does not open the view. The list does not
open it either. The picture can be dropped anywhere on the timeline. Each
picture's center splits the timeline into the places it can land. A drop
between two centers sits between those two pictures: the one on the left
and the one on the right. A drop on a center counts as the left side of
that center, so every point on the timeline is a landing place. A drop at
or left of the first picture's center makes the dropped picture first, and
the old first picture moves to second. A drop right of the last picture's
center makes the dropped picture last. This works before a song is chosen,
while every block has the same width. While the pointer travels, the picture
lifts off the strip and follows it, with a shadow so it reads as held. The
other pictures keep their widths and slide together, leaving a gap the width
of the held picture at the landing place. The rails stay where they are.
Releasing the pointer sets the picture into the gap.

A press on a picture selects it when the pointer does not travel, and
reorders it when the pointer does. A press on the grip in a dissolve
changes the two lengths. The grip is a short vertical mark in the center
of the darker band, and the pointer becomes a left-right arrow over it.
Its grab area is wide enough to take a press comfortably. The mark may be
drawn narrower than that area. The grab area stays out of the center of
each picture, so a short picture can still be selected when the two grips
would otherwise cover it.
The rest of the band still shows the dissolve and still belongs to the
pictures underneath, so selecting or reordering a picture stays possible
there. Drag the grip sideways and time moves from one neighbor to the
other. The film's length stays. Both blocks follow the pointer, because
their widths are those times. Releasing the pointer updates the open
project, which is then unsaved until Save. One picture has no band. Its
length is the song, and it is not dragged.

Classic and Photo film stay `fit-audio`. The drag stores a `dur` pin on
each of the two pictures, changed by opposite amounts, so every other
picture keeps the length it had. One picture in the file still has to stay
unpinned and absorb the song. When one of the two is that picture, only the
other is pinned. With only two pictures, one pin is enough. The drag stops
when either picture would become as short as the dissolve. Clearing the
length box, or Remove custom time, still puts that picture back in the share.

Quick slides is `fixed-clip`. Every picture has one length, and the film is
not the song. The first divider drag leaves that policy: it writes each
picture's current length as `dur`, sets `timing` to `explicit`, then applies
the same exchange. Applying Quick slides again clears those lengths and
returns to one shared clip. Until a fit-audio Look is applied, the length
box keeps a number. There is no share to return to.

### Screens

One screen. New or Open, the song, the picture list, three Looks with one
selected, the timeline, and Save, Export shell script, and Render.
A line under the timeline says how it will come out, in plain words: how many
pictures, about how long each one stays, and that it dissolves between them.
That is the check before a render. It does not mention zoom span or fill zoom.

More, when it is built, is where the other presets are: more Looks, the fill
names, the transition names. Still names, still one choice each.

The picture row starts on the move names and the length box. More on that
picture will be the two stills, enter and exit, a drag point, and the zoom
choices. Hold and a different transition after this picture sit there too.
The window detaches the picture when those change, and keeps the old preset
name visible.

Render progress is the ffmpeg log, unfiltered. The window does not invent a
percent complete from a swallowed stderr.

### Still to add to version 1

The backdrop kinds above are in the specification and the engine. An omitted
backdrop stays the blurred photo.

Joins beyond `fade` are allowed to land in version 1, one plain preset at a
time, starting from a cut or a dip to black if either is wanted. The set is
not chosen yet.

Creative rotation and skew are out of version 1. Metadata rotation stays.
A later Montage is where a creative rotation or a skew would be added, as a
named preset if they are added at all.

## Hosts

The window and the local browser are two hosts of one Svelte client. Both
talk to the Go engine over HTTP at `/v1`. The engine packages stay free of
HTTP and of Wails. Handlers in `web/v1` call `kbvmaker/v1` the way the
CLI does.

Wails is a window onto that server, plus the native open and save dialogs a
browser cannot provide. It does not grow a second set of engine calls.

The window does not send `/v1` with fetch. WebKitGTK before 2.40 drops POST
bodies on the Wails asset server, so a save arrives empty. One binding,
`API.Call(method, path, body)`, hands the request to the same `/v1` handler
and returns its status and body. The client builds the same requests in
both hosts; only the transport differs. This path is kept on every platform,
not only old Linux, so the window has one way to reach `/v1` and the tests
cover it.
Server-sent events carry the ffmpeg log, so render progress is the same in
both hosts.

This version's browser app is that local server. ffmpeg runs on the machine
where the server runs. A person at this version will prefer the window. The
browser is how the same screens are exercised, and how the later site stays
the same client.

A public multi-session site is not this version. There is no login, no user
table, and no session middleware. Each request names a Montage document.
The engine already takes the project directory from that file. Later, a
session layer can sit in front of these handlers and own a directory per
session. The screens and `/v1` stay put.

A browser file input does not give the local server a filesystem path. This
version's browser host sends the path the person typed, because the server
is on that machine and the files are already on disk. The window uses a
native dialog and sends the same kind of path. Uploading bytes into a
session directory belongs to the multi-session site.

Versions split by tree. `web/v1` and `frontend/src/lib/montage/v1` are
Montage 1. They do not import a future v2. Shared code is the app frame,
the design system, and `frontend/src/lib/platform` (dialogs and other host
differences). The shell reads `montage` and mounts that version's editor.
An unrecognized version is a plain message that this build cannot open it.

```
web/v1/                          HTTP, rule errors as JSON
frontend/src/lib/components/ui/  shadcn-svelte
frontend/src/lib/platform/       browser and window hosts
frontend/src/lib/montage/v1/     client, presets, screens
frontend/src/routes/             shell only
```

The UI is SvelteKit, Svelte 5, and Tailwind 4, with shadcn-svelte. The build
is a static client. Kit routes do not resolve a Montage. Buttons, dialogs,
menus, and forms come from shadcn-svelte. The filmstrip, the Look row, and
the enter/exit stills are custom components in the v1 editor.

`/v1` returns the rule error as `{ "slot", "code", "message" }`. Tests cover
the handlers without a window, and the v1 client and screens without Wails.

## Mobile Discussions

I would keep the Montage file and the Go engine, and rebuild the phone around a
project folder and a different editor. The current window is a wide,
mouse-driven screen sitting on ffmpeg, and that combination does not survive a
shrink to an iPhone.

The engine already has the right seam. `v1` resolves the film, and the Svelte
client only talks to it through `/v1`. On the desktop that call is already
in-process (`API.Call`) because the window cannot rely on a normal HTTP POST.
An iPhone build would do the same thing: compile those Go packages into a
static library and call plan, validate, and save from Swift. The timing rules,
looks, cards, and the `tracks` list would not be rewritten. A second copy of
Montage in Swift is how the phone and the desktop would drift apart.

The project folder is the other piece that travels. Pictures and soundtrack
files are already paths relative to the Montage, and the folder is meant to
move with its media. On a phone that folder lives in the app sandbox or in
iCloud Drive. **Add pictures** copies from the photo library into that folder.
**Add soundtrack** copies from Files. The document never stores a photo-library
identifier. Open and Save go through the document picker, and the
unsaved-changes question stays the same one the window already asks.

Rendering is the break. The desktop writes an ffmpeg command and runs it. iOS
has no shell and no ffmpeg on the path, and a 1920×1080 zoom-and-dissolve
render will also thermal-throttle a phone. I would not start by linking libav
into the app. The first phone build would edit and save the Montage, show the
plan, and leave **Render video** to the Mac or the command line, which already
read that file. A phone that must produce the video itself wants a second
backend behind the same plan: AVFoundation compositions for the clips,
dissolves, and the soundtrack concat. That is a new renderer, not a port of
the argv builder. Card painting can stay in Go. Photo thumbnails would switch
from the ffmpeg thumbnail call to ImageIO, since the phone would no longer
have ffprobe.

The screen would be redesigned, not reflowed. One column cannot hold the
project, the soundtrack, the look, the picture list, and both timeline lanes.
The picture list is already the reliable editor: select, length, move, pin,
replace. That becomes the main surface. The film strip becomes a horizontal
scroll of fixed-size thumbnails with a time ruler, because proportional blocks
fall under a usable touch size as soon as the film is long. The dissolve grip
and the silence grip are mouse tools. On a phone those edits stay in the
fields that already exist, **Length in seconds** and **Silence before**.
Drag-to-reorder can remain on the rows. The full-picture preview is already a
tap, so that stays.

So the sequence would be: Go library in an iOS shell, project folder plus
photo and file import, a list-first editor that reads and writes the same
Montage, and render left on the machines that have ffmpeg. A later
AVFoundation renderer is what would make **Render video** honest on the phone.

## Build - Windows

GitHub Actions is still the way to ship Windows binaries. You do not need a
Windows machine for it.

`kbvmaker.exe` is ordinary Go. `kbvmaker-gui.exe` is Wails and needs a real
Windows toolchain, so it is built on a `windows-latest` runner with
`scripts/build.ps1`. ffmpeg stays on the user's PATH. The runner skips that
check, because compiling the programs does not run ffmpeg.

The workflow is in `.github/workflows/windows.yml`. It installs Go 1.23 and
Node 22, builds both executables, and packs them with a short note into
`kbvmaker-windows-amd64.zip`.

- Pushing a tag shaped like `v1.0.0` publishes that zip on the GitHub release
  for the tag.
- Running the workflow by hand from the Actions tab builds the same zip and
  keeps it as an artifact, with no release. That is the way to try the job
  before the first tag.

The workflow file has to be on the commit you tag. After you commit and push
these changes:

```
git tag v1.0.0
git push origin v1.0.0
```

The release then appears on https://github.com/neallawson/kbvmaker/releases.
I have not committed or pushed this.

## Future Zoom/Pan Enhancements

Two controls, both already resolved by the engine. The seven named moves stay
the presets. The window today only assigns those moves.

**How far.** A picture can say how far its move zooms. That amount is
`zoomSpan`. For a move that travels the whole picture, the end zoom is
`min(fill, 1 + zoomSpan)`. The pace stays coupled to the picture's length, so
a long picture and a short one move at the same speed and stop when the photo
fills the frame, or at `1 + zoomSpan` when that is nearer. The project default
`0.4` is an end near `1.4`, capped by `fill`. The document already stores
`zoomSpan` on the defaults, and a flag can override it for one run. The
window does not yet offer it on a picture.

**Where it aims.** A picture can name the spot the move travels toward. The
point is chosen on the full-image preview. The document stores it as the pan
point it already resolves: a named place (`center`, `left`, `right`, `top`,
`bottom`, or a corner) or a `u,v` in `0..1`. The preset keeps its name. Slow
zoom in stays Slow zoom in, aimed at that spot. Direction is the label for
the two points, not a separate field.

## Appendix

### Research into zoom anti-aliasing

Some zooms hitch. The picture sits, then steps. How obvious it is depends
on the photograph and on how far the zoom travels. This was measured on
26 October 2026 against ffmpeg `N-123918-gf7ca6f7481-20260411` and left
out of the renderer. The app still crops the 3840×2160 still straight to
1920×1080.

A smaller zoom increment per frame, played faster so the film stays the
same length, does not help. The zoom is already a continuous ramp in
seconds, one sample per output frame. Extra in-between frames, sped up
with `setpts` to keep the duration, are the frames a 30 fps render would
have dropped. The ones that remain land on the same pixel grid.

`zoompan` rounds the crop window to a whole pixel of the still it reads,
then scales that window to the output. A slow zoom often moves the window
by less than one pixel between frames, so several frames are identical and
the next frame jumps. A portrait, a wide photo, and a deeper zoom each
turn that one-pixel jump into a different amount of the picture.

The canvas is already 3840×2160. Raising `zoompan`'s output size to that
canvas and scaling back to 1920×1080 with Lanczos copies the same rounded
window into a larger frame. The downscale does not invent the positions
between those pixels.

The finer step is to enlarge the still first. At 7680×4320, one rounded
pixel is half as large on the 1920×1080 frame, and the window moves about
twice as often. `zoompan` still writes 1920×1080 at 30 fps, so the film
length stays the same.

A 384×216 grid, zooming from 1 to 1.02 over 2 seconds at 30 fps, shows the
hold. A 3840×2160 `testsrc2` still, zooming from 1 to 1.15 over 3 seconds,
shows the cost. Clips were libx264, preset `ultrafast`, CRF 20.

| Path | Time for 3 seconds of video | Longest run of identical frames |
| --- | --- | --- |
| Current: crop the 3840×2160 still straight to 1920×1080 | 1.9 s | 8 |
| `zoompan` at 3840×2160, then Lanczos down to 1920×1080 | 4.6 s | 8 |
| Scale the still to 7680×4320, then `zoompan` to 1920×1080 | 3.6 s | 4 |

The middle path took about 2.4 times the clip work and kept the 8-frame
hold. The last path took about 1.9 times the clip work and cut that hold
to 4 frames. The three files were the same size, about 7 MB for that
3-second sample.

On a real film the clip stage would grow by about that much. Joins, the
soundtrack mux, the 1920×1080 size, the frame rate, and the duration would
stay as they are. A render that spends most of its time in the clips gets
close to twice as slow. A render with many dissolves spends more of its
time joining those clips, so the whole job grows by less. An 8K frame is
about 128 MB of RGBA. Another doubling, to 15360×8640, would halve the
remaining step again and push a frame toward 500 MB.

The shots that would improve are the slow ones: a long center zoom, a
portrait that only creeps, fine detail such as foliage, brick, and hair.
A zoom that already moves at least one canvas pixel per frame would look
the same. The photograph would not get sharper. A small image fitted up
to the canvas is already soft, and enlarging it again only refines the
motion. Lanczos, which the canvas fit already uses, can leave a faint
halo on a hard edge. A drifting card would get the same smaller step. A
card that holds still would be unchanged.

Other ideas that were set aside with this one: 60 fps makes the jumps
more frequent and leaves their size alone. Easing makes the slow ends
sit on one pixel for longer. Frame blending and motion interpolation
smear the step and read as softness on a photograph.
