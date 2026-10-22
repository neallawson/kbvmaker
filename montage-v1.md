# Montage v1

Normative specification of the project document that the kbvmaker engine
loads, resolves, and renders. `IDEAS.md` is the design record. Where the two
disagree, this document wins.

Montage is a **composition format**: a versioned JSON document with a closed
set of keys and values, plus a resolution procedure. It describes still
images, a soundtrack, the cuts between them, and the motion inside each
picture. It is not a scripting language. It has no expressions, variables, or
control flow. Easing is an enum. A later version may add names; it does not
add a grammar.

Ken Burns moves are the built-in modes in section 10. The format also holds holds, pans,
zooms, dissolves, and cut lists that have nothing to do with that style.

The application is kbvmaker. The format is Montage. Another engine may
implement this document.

## 1. Conformance

A **document** conforms when it is JSON matching the types in this spec and
every condition in Checks holds after resolution.

An **engine** conforms when it:

- accepts every conforming document and rejects every non-conforming one
- resolves time, zoom, and pan by the rules in this spec
- renders zoom `1.0` as the whole photograph, by the frame model below
- names the slot and the rule when it rejects a document

JSON objects have no duplicate keys. The engine rejects a document with a
duplicate key, an unknown key, a missing required key, or a value of the
wrong type. Unknown keys are not ignored.

Readers accept keys in any order. When the engine writes a document (including
a baked schedule), it pretty-prints UTF-8 JSON with two-space indentation, a
trailing newline, and the key order given in each object below.

## 2. File and paths

The usual filename is `name.montage.json`.

Paths in the document are relative to the directory that contains the
document. Absolute paths are allowed. A path is a filesystem path. URLs are
not. The engine rejects a path that does not exist at plan time.

A pool list file (see Pool) is UTF-8 text. One path per line. A line whose
first non-whitespace character is `#`, and a line that is empty or
whitespace, is ignored. Paths in that file are relative to the directory that
contains the list file.

## 3. Units

| Value | Unit |
| --- | --- |
| `duration`, `hold`, `dur`, `offset`, `sec`, `clip` | seconds |
| `pct` | percent of the named clock, from 0 through 100 |
| `timecode` | `HH:MM:SS` or `HH:MM:SS.mmm` from the start of the song |
| `frame` | frame index from the start of the song, at `output.fps` |
| `zoom`, `zoomSpan`, `full`, `fill`, `limit` | unitless zoom; `1.0` is the whole photograph |
| `u`, `v` | point on the photograph, 0 through 1 |
| `width`, `height` | pixels |
| `fps` | frames per second |
| `blur` | Gaussian sigma at the canvas size |
| `color` | `#RRGGBB` |
| `crf` | encoder quality |
| `audioBitrate` | bits per second |

A bare number on a duration field is seconds. Percent, timecode, frames, and
anchors are the spellings that name a clock.

## 4. Document

```
Document {
  montage: 1
  canvas?:   Canvas
  output?:   Output
  defaults?: Defaults
  audio:     string
  timing?:   Timing
  pool?:     Pool
  schedule?: Schedule
  modes?:    { [name]: ModeDef }
  moves?:    string[]
  slots?:    Slot[]
}
```

`montage` must be the integer `1`.

`audio` is the soundtrack. The song length is the container duration reported
by ffprobe. The editor may omit `audio` while a project is still being
assembled. `plan` and `render` also accept a document with no audio when the
timing policy has its own clock: `fit-audio` with `duration`, `fixed-clip`
with authored slots, or `explicit` with authored slots. A pool, `markers`,
and `fit-audio` without `duration` still need a song.

The document has `slots`, or `pool`, or both only in the way Baking
describes. The editor may omit both while a project is still being
assembled. `plan` and `render` reject a document with neither. `schedule`
is present only when `pool` is present.

`timing` omitted means `{ "policy": "fit-audio" }`.

`moves`, when present, is the list of mode names Shuffle may assign, in
the order a Look cycles them. Each name is a built-in mode or a key of
`modes`, and each name appears once. Choosing a move for one picture may
use any built-in mode. Plan and render do not read `moves`. An omitted
`moves` means Shuffle may assign every built-in mode.

### 4.1 Canvas

```
Canvas { width?: number, height?: number, blur?: number, backdrop?: Backdrop }

Backdrop {
  kind: "blur" | "color" | "pattern" | "image"
  blur?:  number
  color?: string
  name?:  string
  src?:   string
}
```

Defaults: width `3840`, height `2160`, and a `blur` backdrop with sigma `40`.

Width and height are positive even integers. Each canvas side is greater than
or equal to the matching output side.

`backdrop` is the area the fitted photograph does not cover. `fill` remains
the zoom that covers the frame. An omitted `backdrop` is `{ "kind": "blur",
"blur": 40 }`. `canvas.blur` sets that sigma when `backdrop` is omitted.
Setting both `blur` and `backdrop` is rejected.

`kind` selects the bars:

- `blur` cover-crops the same photograph and blurs it. `blur` defaults to
  `40` and is `>= 0`. `color`, `name`, and `src` are rejected.
- `color` is a flat color. `color` is required, `#RRGGBB`. The other fields
  are rejected.
- `pattern` is a shipped drawing. `name` is `grain` or `gradient`. `color`
  defaults to `#000000`. `grain` is uniform noise on that color, strength
  `20`, seed `1`. `gradient` runs from that color at the top to black at the
  bottom, speed `0`, seed `1`. `blur` and `src` are rejected.
- `image` cover-crops `src`, a path relative to the document. `blur` defaults
  to `0`. `color` and `name` are rejected.

A slot may set `backdrop`. An omitted slot backdrop uses the canvas backdrop.

### 4.2 Output

```
Output {
  width?:        number
  height?:       number
  fps?:          number
  file?:         string
  videoCodec?:   string
  crf?:          number
  preset?:       string
  pixFmt?:       string
  audioCodec?:   string
  audioBitrate?: number
}
```

Defaults: width `1920`, height `1080`, fps `30`, videoCodec `libx264`,
crf `20`, preset `medium`, pixFmt `yuv420p`, audioCodec `aac`,
audioBitrate `192000`.

Width and height are positive even integers. `fps` is a positive integer.
`file` is the output path. The engine may take the same values from
invocation flags; a flag wins over the document.

### 4.3 Defaults

```
Defaults {
  join?:     Join
  hold?:     number
  zoomSpan?: number
  ease?:     "linear"
}
```

Defaults: join `{ "type": "fade", "duration": 1.5 }`, hold `1.0`,
zoomSpan `0.4`, ease `linear`.

`hold` is `>= 0`. `zoomSpan` is `> 0`. Version 1 has one ease, `linear`.

### 4.4 Join

```
Join { type: "fade", duration: number }
```

`duration` is `> 0`. `type` has one value in version 1, `fade`. Further
transition names will be new values of `type`.

On a slot, `join` is the dissolve that follows that slot. The last slot's
`join` is rejected. An omitted slot join uses `defaults.join`.

## 5. Time

A time value is a position on a clock, or a length.

Clocks:

- `song` — the soundtrack.
- `clip` — one slot, including its fades.
- `opaque` — the part of that clip where the picture is fully visible.

```
Clock  = "song" | "clip" | "opaque"
Anchor = "song-start" | "song-end" | "clip-start" | "clip-end"
       | "opaque-start" | "opaque-end"

TimePoint =
    { sec: number, of: Clock }
  | { pct: number, of: Clock }
  | { timecode: string }
  | { frame: number }
  | { anchor: Anchor, offset?: number }
```

`pct` is from 0 through 100. `100` is the end of the clock. `frame` is an
integer `>= 0` and converts as `frame / output.fps` seconds from the start of
the song. `timecode` and `frame` are song positions; they have no `of` field.

`offset` is seconds added to the anchor, and it may be negative. Omitting
`offset` means `0`.

A negative `sec` counts back from the end of that clock. `{ "sec": -2, "of":
"song" }` is two seconds before the audio ends. A negative `pct` is rejected.

Travel is not a clock. The length `rest` means "through `clip-end`."

### 5.1 Which spelling to write

Seconds from the start of a clock are an absolute position.
`{ "sec": 92.4, "of": "song" }` is 92.4 seconds from the start of the audio.

Percent is the same kind of position, relative to the clock's measured
length. `{ "pct": 41, "of": "song" }` is `0.41` times the song duration. A
percent tracks a new duration of the same arrangement. A raw second stays on
the original spot.

`{ "pct": 50, "of": "clip" }` is halfway through that clip, fades included.
`{ "pct": 50, "of": "opaque" }` is halfway through the fully visible stretch.

An anchor plus an offset is relative to an event.
`{ "anchor": "opaque-start", "offset": 1.0 }` is one second after the picture
is fully visible. It stays put when the song length changes or a neighboring
cut moves. It moves when this slot's join changes. A song percent inside the
slot moves with the cut.

`{ "dur": 1.0 }` on a segment is one second from the end of the previous
segment, or from `clip-start` when the segment is first. `{ "dur": "rest" }`
runs to `clip-end`.

Bars and beats are not in version 1. They need a tempo map, which the audio
file does not carry.

## 6. Slots, segments, modes

```
Slot {
  src:       string
  mode?:     string
  motion?:   Segment[]
  basedOn?:  string
  hold?:     number
  join?:     Join
  dur?:      number
  until?:    TimePoint
  backdrop?: Backdrop
}

Segment {
  from?: TimePoint
  to?:   TimePoint
  dur?:  number | "rest"
  zoom?: ZoomSpec
  pan?:  PanSpec
  ease?: "linear"
}

ModeDef { family: "reveal" | "scan", motion: Segment[] }
```

A slot has `mode` or `motion`, exactly one. `motion` has at least one
segment. `basedOn` is allowed only with `motion`. The engine does not render
`basedOn`. It records which catalog row the segments were copied from when a
slot was detached from a mode.

`hold` is allowed only with `mode`. It replaces `defaults.hold` when that
mode is expanded. `dur` on a slot is a clip length in seconds. It is
required under `explicit`. Under `fit-audio` it is optional and means that
picture's length is pinned; slots that omit it share the rest of the song,
or the rest of `timing.duration` when there is no soundtrack (section 7.1).
Under `fixed-clip` it is optional and replaces `clip` for that slot only
(section 7.2). `until` is allowed only under `markers`.

Mode names, including keys of `modes`, match `[a-z0-9]+(-[a-z0-9]+)*`.

A name in `modes` replaces a built-in of the same name for that document.
`family` is required on a project mode.

### 6.1 Zoom

```
ZoomAtom     = number | "full" | "fill" | "limit"
ZoomEndpoint = ZoomAtom | { min: ZoomAtom[] }
ZoomSpec     = ZoomEndpoint | { from: ZoomEndpoint, to: ZoomEndpoint }
```

`min` has at least one element. A number is `>= 1`.

- `full` is `1`.
- `fill` is the per-photo zoom that just covers the canvas aspect. See Frame
  model.
- `limit`, on a segment of duration `d` in a clip of duration `clipDur`, is
  `1 + (zoomSpan / clipDur) * d`.

`{ "min": [...] }` resolves each atom and uses the smallest, then raises the
result to `1` if it would fall below. The default far end of a built-in move
is `{ "min": ["fill", "limit"] }`.

A `ZoomSpec` object with `from` and `to` interpolates. Any other `ZoomSpec`
is constant for the segment. Speed is never stored. `zoomSpan` is the zoom
change that would accumulate if a move lasted the whole clip; the segment's
own duration scales `limit`.

### 6.2 Pan

```
PanName = "center" | "left" | "right" | "top" | "bottom"
        | "top-left" | "top-right" | "bottom-left" | "bottom-right"
PanAtom = { u: number, v: number } | PanName
PanSpec = PanAtom | { from: PanAtom, to: PanAtom }
```

`u` and `v` are from 0 through 1. Named points:

| Name | u | v |
| --- | --- | --- |
| `top-left` | 0 | 0 |
| `top` | 0.5 | 0 |
| `top-right` | 1 | 0 |
| `left` | 0 | 0.5 |
| `center` | 0.5 | 0.5 |
| `right` | 1 | 0.5 |
| `bottom-left` | 0 | 1 |
| `bottom` | 0.5 | 1 |
| `bottom-right` | 1 | 1 |

A named point is that point on the photograph, not a pixel on the canvas.
The engine clamps it into the visible window at the sample's zoom. At zoom
`1` every point clamps to the center, so the frame stays put while the whole
photograph is showing.

### 6.3 Segment ends

A segment has `to` or `dur`, exactly one. `from` may be omitted.

- Omitted `from` means the previous segment's end. The first segment starts
  at `clip-start`.
- `dur` as a number is a length in seconds from `from`.
- `dur` of `rest` means `to` is `clip-end`.
- Resolved `to` is greater than resolved `from`.

Segments are in order and do not overlap. A gap holds the zoom and pan of
the previous segment. The gap before the first segment, if `from` is after
`clip-start`, holds zoom `full` and pan `center`. An omitted `zoom` or `pan`
repeats the value at the start of the segment, which is the previous end, or
`full` and `center` at the start of the clip.

`ease` omitted means `defaults.ease`.

Inside a segment, zoom and the subject point each interpolate linearly in
segment-local time. The engine clamps pan after interpolation, at each
sample's zoom, so a linear subject can produce a pan that pauses while the
window is still against the canvas edge.

## 7. Timing policies

`JoinDur(i)` is the duration of the join between slot `i` and slot `i+1`
(0-based). Slot-local time is seconds from the start of that clip.

Opaque local range of a slot:

- `opaque-start` = incoming join duration, or `0` when the slot is first
- `opaque-end` = `clipDur -` outgoing join duration, or `clipDur` when the
  slot is last

The first slot has no fade in. The last has no fade out. A hold written from
`opaque-start` does not invent a fade the slot does not have.

### 7.1 fit-audio

`N` is the number of slots. `D` is the song length. When `audio` is omitted,
`D` is `timing.duration` instead:

```
Timing { policy: "fit-audio", duration: number }
```

`duration` is `> 0`. A document that sets both `audio` and `duration` is
rejected: the song and the typed film length cannot both be the clock.
`x` is `JoinDur` when every join has that same duration. Version 1 of
`fit-audio` requires every join to share one duration; per-slot join
overrides belong to `markers` and `explicit`.

```
clipDur = (D + (N - 1) * x) / N
```

A slot may set `dur` to a clip length in seconds, `> 0`. That length is a
pin. The file stores only the pins. The shared lengths are recomputed and
are not written back.

Let `P` be the sum of the pins and `k` the number of pins. When `N > 1`,
`k` must be less than `N`, so one picture still absorbs the song, or the
typed film length when there is no soundtrack:

```
shared = (D + (N - 1) * x - P) / (N - k)
```

Every pin is `> x`, and `shared` is `> x`. Slots that omit `dur` use
`shared`. When `k` is 0 this is the even split above.

When `N` is 1 the clip is `D`. A `dur` on that only slot is rejected unless
it equals `D`. With no soundtrack the messages name the film: the only
picture runs for the whole film, and one picture must keep sharing the film.

On the output timeline the fade between slot `i` and slot `i+1` starts at
the next slot's start and lasts `x`. With equal clips that start is
`(i + 1) * (clipDur - x)`. The authored quantity is the clip length, not
the midpoint of the dissolve.

Output duration is the sum of the clip lengths minus `(N - 1) * x`, which
equals `D`.

### 7.2 fixed-clip

```
Timing { policy: "fixed-clip", clip: number }
```

`clip` is `> 0` and is the clip duration of every slot that omits `dur`.
A slot may set `dur` to its own clip length, `> x` when `N > 1`. The other
slots stay at `clip`. The resolved output duration is then the sum of those
lengths minus the joins, which is `N * (clip - x) + x` when every slot omits
`dur`. Authored slots do not need a soundtrack. With an authored `slots`
array, `N` is the length of that array. With a `loop` schedule and no
authored slots, `N` falls out of the song, and `dur` on a generated slot is
ignored:

```
N = max(1, round((D - x) / (clip - x)))
```

`clip` must be `> x`. When a song supplied `N`, the plan reports both the
video duration and that song length. The fade chain uses the same
start-of-fade rule as `fit-audio`.

`given` and `shuffle` produce one slot per pool item. `fixed-clip` with
those schedules is legal when that count equals `N` from the formula above.
Otherwise the engine rejects the document.

### 7.3 explicit

Each slot's `dur` is its clip duration in seconds, `> 0`. Output time of the
start of slot `0` is `0`. The start of slot `i+1` is

```
start[i] + dur[i] - JoinDur(i)
```

Output duration is the sum of the clip durations minus the sum of the joins.

### 7.4 markers

Each slot except the last has `until`, a time point on the `song` clock. The
last slot has no `until` and runs to the end of the song. `until` is the
middle of the dissolve.

Cut points are `C[0] = 0`, then each `until` in order, then `C[N]` at the
song length. After resolution the cuts are strictly increasing, and every
`until` lies strictly between `0` and `D`.

With `x` the join duration at that cut:

- the first clip runs from `0` to `C[1] + x/2`
- a middle clip runs from `C[i] - x/2` to `C[i+1] + x/2`
- the last clip runs from `C[N-1] - x/2` to `D`

Neighbors overlap by exactly `x`, centered on the cut. A middle picture is
fully visible from `C[i] + x/2` to `C[i+1] - x/2`.

Neighboring cuts may mix `sec`, `pct`, `timecode`, and `frame`.

## 8. Pool and schedule

```
Pool =
    { dir: string, sort?: "natural" | "lexical" }
  | { list: string }
  | { files: string[] }

Schedule { policy: "given" | "shuffle" | "loop", seed?: number }
```

`sort` omitted means `natural`. `lexical` compares paths as raw bytes.
`natural` compares the same way, except a run of ASCII digits compares as a
base-10 integer. `Greenhouse2.jpg` sorts before `Greenhouse10.jpg`. When two
digit runs have the same integer value, the shorter run comes first. A tie
on the whole name breaks by raw bytes.

`dir` is non-recursive. The engine includes regular files whose extension,
compared case-insensitively, is `jpg`, `jpeg`, `png`, `tif`, `tiff`, `webp`,
or `bmp`. Dotfiles are skipped. `files` is an authored order. `list` is the
text file from File and paths.

`seed` is an integer from `0` through `2^53 - 1`. It is required on
`shuffle` and `loop`, and rejected on `given`.

`given` emits the pool in sort order, or in list order, one slot each, mode
`in-center` unless the document says otherwise. A pool document has no
per-item mode. Modes are assigned after baking, on the slots. Generated
slots all use `in-center`.

`shuffle` emits one permutation of the pool.

`loop` emits `N` draws with no immediate repeat. The pool must contain at
least two files. `N` comes from `fixed-clip`, or from `timing.count` when
the policy is `fit-audio`:

```
Timing { policy: "fit-audio", count?: number }
```

`count` is a positive integer and is legal only with `pool` and `loop`.

### 8.1 Random draws

The generator is splitmix64. `state` starts at `seed`.

```
next():
  state = state + 0x9E3779B97F4A7C15   mod 2^64
  z = state
  z = (z xor (z >> 30)) * 0xBF58476D1CE4E5B9   mod 2^64
  z = (z xor (z >> 27)) * 0x94D049BB133111EB   mod 2^64
  return z xor (z >> 31)
```

An integer in `[0, n)`, for `n > 0`, is rejection-sampled: draw `next()`
until the value is `< floor(2^64 / n) * n`, then reduce modulo `n`. All
arithmetic is on unsigned 64-bit integers.

`shuffle` is Fisher-Yates from the last index down to `1`. At index `i`,
swap `i` with `nextInt(i + 1)`.

`loop` draws each slot from the pool indexes other than the previous draw.
The first draw is `nextInt(poolSize)` over the whole pool.

### 8.2 Baking

Baking writes the generated slots into `slots`, removes `pool` and
`schedule`, and leaves the timing policy in place. A later resolve does not
draw again. Render uses `slots` and does not read `pool`.

## 9. Frame model

Zoom `1` means the entire photograph is in the frame. The engine does not
crop to the output aspect before the move.

Probe width and height are the displayed size: the engine applies a 90° or
270° display rotation (ffprobe display matrix, or the rotate tag) before
computing `fill` and before building the canvas. ffmpeg autorotate on the
canvas input agrees with that probe.

`fill` for a photograph of displayed size `w × h` on a canvas `W × H`:

```
if w/h >= W/H then fill = (w/h) / (W/H)
else               fill = (W/H) / (w/h)
```

The canvas still is built once per source path and effective backdrop, and
reused for every slot with that pair. The sharp branch is always the slot's
photograph, scaled to fit inside `W × H`, `setsar=1`, then centered on the
backdrop. One frame is kept. The motion filter reads that still.

The backdrop branch depends on `kind`:

- `blur`: scale the same photograph to cover `W × H`, crop, Gaussian blur of
  the backdrop sigma, `setsar=1`.
- `color`: a solid frame of that size in the given color.
- `pattern`: `grain` or `gradient`, drawn at `W × H` as specified on
  `Backdrop`.
- `image`: scale `src` to cover `W × H`, crop, then the optional blur.

A sigma of `0` skips the blur filter. Display rotation on an `image`
backdrop is ffmpeg's autorotate on that input.

Let the fitted sharp rectangle be width `dw`, height `dh`, origin
`(x0, y0)` inside the canvas.

At clip-local time `t` the resolved sample is zoom `z` and subject `(u, v)`.
The window in the canvas is `W/z` by `H/z`. Its top-left `(x, y)` is:

```
px = x0 + u * dw
py = y0 + v * dh
x  = clamp(px - W/(2z),  0, W - W/z)
y  = clamp(py - H/(2z),  0, H - H/z)
```

The motion filter is `zoompan` with `z`, `x`, `y`, one output frame per
input frame, size `output.width × output.height`, at `output.fps`. Samples
are continuous in clip-local seconds; `zoompan`'s `time` variable is that
clock. The document never stores a `zoompan` expression.

## 10. Built-in modes

`H` stands for the slot's `hold`, or `defaults.hold`. Every built-in except
`hold` is family `reveal`. Expansion uses these segments, then resolution
proceeds as for authored `motion`.

`in-center`:

```json
[
  { "from": { "anchor": "opaque-start" }, "dur": H,
    "zoom": "full", "pan": "center" },
  { "dur": "rest",
    "zoom": { "from": "full", "to": { "min": ["fill", "limit"] } },
    "pan": "center" }
]
```

`in-left-right` is the same with the travel pan
`{ "from": "left", "to": "right" }`. The hold's pan is `left`, which clamps
to center while the zoom is `full`.

`in-right-left` is the same with the travel pan
`{ "from": "right", "to": "left" }`. The hold's pan is `right`.

`in-top-bottom` is the same with the travel pan
`{ "from": "top", "to": "bottom" }` and the hold's pan `top`.

`in-bottom-top` is the same with the travel pan
`{ "from": "bottom", "to": "top" }` and the hold's pan `bottom`.

`out-center`:

```json
[
  { "from": { "anchor": "clip-start" },
    "to": { "anchor": "opaque-end", "offset": -H },
    "zoom": { "from": { "min": ["fill", "limit"] }, "to": "full" },
    "pan": "center" },
  { "dur": "rest", "zoom": "full", "pan": "center" }
]
```

`hold`, family `reveal`:

```json
[
  { "dur": "rest", "zoom": "full", "pan": "center" }
]
```

`hold` has no moving span, so a short opaque range is legal as long as it is
positive. The other built-in modes travel, so the hold has to fit inside
the opaque range.

Project modes and inline `motion` use the same segment rules. Family `scan`
opts out of the reveal check: the picture may stay at `fill` and only pan.
`scan` is never the family of an unnamed or default mode.

## 11. Resolution order

1. Load the document and apply invocation overrides.
2. Probe the song duration.
3. Generate slots from the pool, unless `slots` is already present.
4. Resolve the timing policy into clip durations and output-time extents.
5. Resolve each slot's opaque range from the joins it actually has.
6. Expand `mode` into segments.
7. Resolve segment endpoints in order to clip-local seconds. A song-time
   point inside a segment is legal when that instant lies inside the slot's
   output extent. Any other point outside the clip is an error.
8. Resolve `fill` from the photograph. Resolve `limit` from the segment
   duration. Interpolate zoom and pan.
9. Run Checks. Then compile the frame model.

Invocation flags may override the audio path, the output path, the join
duration, the hold, the zoom span, the blur, the random seed, and the
schedule policy. The flag wins over the document. The blur flag sets the
sigma of a `blur` backdrop, or of an `image` backdrop. It does not change a
`color` or `pattern` backdrop.

## 12. Checks

The engine rejects the document and identifies the slot by its 1-based index
and the rule.

| Rule | Condition |
| --- | --- |
| `missing-media` | An audio, image, list, or pool path does not exist, or a probed file is not the kind of media the path claims. |
| `bad-number` | A duration, zoom, percent, dimension, or seed is outside its range. |
| `bad-time` | A time point is malformed, or a marker `until` is not on the song clock. |
| `cuts` | Marker cuts are not strictly increasing, or an `until` is at or outside the song. |
| `dissolve` | A gap between marker cuts is not longer than that boundary's join, so the picture would never be fully opaque. Also a join that is not shorter than either clip it overlaps. |
| `no-travel` | A segment that changes zoom or pan has duration `<= 0`, or a traveling built-in mode's hold does not lie inside the opaque range. |
| `reveal` | Family `reveal`, and the minimum zoom on the closed opaque interval is greater than `1.001`. |
| `span` | Segments overlap, a segment runs past the clip, or the clip is not covered and the gap rule cannot fill it because an endpoint failed. |
| `mode` | Both `mode` and `motion` are set, neither is set, the name is unknown, or the name is malformed. |
| `pool` | `loop` has fewer than two pictures, or `given` / `shuffle` cannot supply the `N` that `fixed-clip` requires. |

Zoom after resolution is finite and `>= 1`. A value above the canvas width is
rejected (`bad-number`): the window would be under one pixel wide.

Family `scan` skips the `reveal` check and still passes every other rule.

## 13. Render

The engine renders each slot's canvas through the motion filter for
`clipDur` seconds, joins the clips with `xfade` of type `fade` at the
resolved offsets, and muxes the soundtrack when the document has one.
A document with no soundtrack writes the video stream only.

Video defaults are the Output defaults. Audio is AAC at `audioBitrate` when
a soundtrack is muxed. That output is trimmed to the shorter of the resolved
video duration and the song (`-shortest` in ffmpeg terms). `+faststart` is
set on the file.

The plan, before any encode, reports the song duration when there is a song,
each slot's source,
mode or `basedOn`, family, clip start, clip duration, opaque range, `fill`,
the far zoom, and each join's start and midpoint. That report is the
resolved timeline. It is not written back into the document.

## 14. Examples

Even clips, two built-in modes. Omitted fields take the defaults in this
spec. `duration` and `hold` are seconds.

```json
{
  "montage": 1,
  "audio": "Love_Ruins-Greenhouse.mp3",
  "slots": [
    { "src": "Greenhouse3.jpg", "mode": "out-center" },
    { "src": "Greenhouse8.jpg", "mode": "in-left-right" }
  ]
}
```

Cuts on the song. `18` is a percent. `92.4` is seconds. The last picture
runs to the end of the audio. The number in `until` is the middle of the
fade.

```json
{
  "montage": 1,
  "audio": "Love_Ruins-Greenhouse.mp3",
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
    { "src": "chorus.jpg", "mode": "hold" }
  ]
}
```

A detached slot. The segments are the edit. `basedOn` is a note.

```json
{
  "montage": 1,
  "audio": "song.mp3",
  "slots": [
    {
      "src": "portrait.jpg",
      "basedOn": "in-left-right",
      "motion": [
        {
          "from": { "anchor": "opaque-start" },
          "dur": 1.0,
          "zoom": "full",
          "pan": "center"
        },
        {
          "dur": "rest",
          "zoom": { "from": "full", "to": { "min": ["fill", "limit"] } },
          "pan": { "from": "left", "to": "right" }
        }
      ]
    }
  ]
}
```

## 15. Versioning

A document with `montage` other than `1` is rejected by a version-1 engine.
A future version adds keys or enum values; it does not reuse a version-1
key for a new meaning. Version 1 has no expression language and no musical
clock.

The Go API for this document is the package tree `kbvmaker/v1`. Its
`Version` constant is 1, the same number as the `montage` field, and the
same number as `renderPlan` in an exported command list. A later Montage
is a new package, `kbvmaker/v2`, which accepts `montage: 2`. Version 1
stays able to load a version-1 document. The engine does not accept two
document versions through one package.

Creative rotation and skew are not part of version 1. Metadata rotation is
applied when the image is probed. A later Montage is where a user rotation
or a skew would be added. Version 1 has no fields for them.

This version currently accepts one join type, `fade`, and one ease,
`linear`. Further join types may be added here before the format is treated
as finished. A version-1 engine rejects, by the unknown-key and
unknown-value rules: easing other than `linear`, tempo maps, creative
rotation, skew, and motion whose source is a moving image rather than a
still.
