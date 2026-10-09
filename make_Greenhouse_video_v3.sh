#!/bin/bash
set -euo pipefail

# ============================================================================
# Greenhouse Music Video Generator (v3)
# Ken Burns zoom/pan + crossfade transitions, synced to 4:42 (282 seconds)
#
# Each photo is fitted whole onto a 4K canvas (blurred backdrop in the bars).
# Zoom 1.0 is that whole photo. Every pattern reaches zoom 1.0 while the
# picture is fully opaque, and the other end of the move is capped by how
# far a fixed zoom speed can travel in the remaining time — never past the
# zoom that just fills the 16:9 frame.
# ============================================================================

# ---- CONFIGURATION --------------------------------------------------------
AUDIO="Love_Ruins-Greenhouse.mp3"
OUTPUT="Love_Ruins-Greenhouse.mp4"
FPS=30
WIDTH=1920
HEIGHT=1080
PRE_WIDTH=3840         # zoompan working size; headroom so the punch-in stays sharp
PRE_HEIGHT=2160
XFADE_DUR=1.5          # crossfade duration in seconds
TARGET_DUR=282         # 4:42 in seconds
FULL_ZOOM=1.0          # whole photograph visible, bars and all
ZOOM_SPAN=0.4          # zoom units the move would cover across a full clip; sets the speed
HOLD_DUR=1.0           # seconds the whole photo stays up while fully opaque, besides the crossfade
BLUR_SIGMA=40          # backdrop blur at the 4K working size

# Image list in the exact order provided
IMAGES=(
  "Greenhouse1.jpg"
  "Greenhouse2.jpg"
  "Greenhouse3.jpg"
  "Greenhouse4.jpg"
  "Greenhouse5.jpg"
  "Greenhouse6.jpg"
  "Greenhouse7.jpg"
  "Greenhouse8.jpg"
  "Greenhouse9.jpg"
  "Greenhouse10.jpg"
  "Greenhouse_band_live.jpg"
  "Greenhouse_band_shot.jpg"
  "Greenhouse_band_tape_IMG_3639_09-10-2026-rotated.jpg"
)

NUM_IMAGES=${#IMAGES[@]}

# ---- TIMING CALCULATION ---------------------------------------------------
# total = N * clip_dur - (N-1) * xfade_dur
# so: clip_dur = (total + (N-1) * xfade_dur) / N
CLIP_DUR=$(awk "BEGIN { printf \"%.4f\", ($TARGET_DUR + ($NUM_IMAGES - 1) * $XFADE_DUR) / $NUM_IMAGES }")
XFADE_STEP=$(awk "BEGIN { printf \"%.4f\", $CLIP_DUR - $XFADE_DUR }")
# Speed is ZOOM_SPAN over a whole clip. The move itself only runs outside the
# crossfade and the full-frame hold, so the far zoom is speed * travel time.
ZOOM_SPEED=$(awk "BEGIN { printf \"%.6f\", $ZOOM_SPAN / $CLIP_DUR }")
TRAVEL_DUR=$(awk "BEGIN { printf \"%.4f\", $CLIP_DUR - $XFADE_DUR - $HOLD_DUR }")
ZOOM_LIMIT=$(awk "BEGIN { printf \"%.4f\", $FULL_ZOOM + $ZOOM_SPEED * $TRAVEL_DUR }")
# Zoom-in patterns sit on the whole photo through the fade-in, then this long again.
T_HOLD_IN=$(awk "BEGIN { printf \"%.4f\", $XFADE_DUR + $HOLD_DUR }")

if ! awk "BEGIN { exit !($TRAVEL_DUR > 0) }"; then
  echo "ERROR: clip ${CLIP_DUR}s is shorter than crossfade ${XFADE_DUR}s plus hold ${HOLD_DUR}s"
  exit 1
fi

echo "=== Greenhouse Music Video ==="
echo "Images: $NUM_IMAGES"
echo "Clip duration each: ${CLIP_DUR}s"
echo "Crossfade duration: ${XFADE_DUR}s"
echo "Xfade offset step: ${XFADE_STEP}s"
echo "Zoom speed: ${ZOOM_SPEED}/s"
echo "Zoom travel: ${TRAVEL_DUR}s"
echo "Zoom limit: ${ZOOM_LIMIT} (whole photo at ${FULL_ZOOM})"
echo "Target total: ${TARGET_DUR}s"
echo ""

# ---- VERIFY INPUTS --------------------------------------------------------
for img in "${IMAGES[@]}"; do
  if [ ! -f "$img" ]; then
    echo "ERROR: Image not found: $img"
    exit 1
  fi
done
if [ ! -f "$AUDIO" ]; then
  echo "ERROR: Audio file not found: $AUDIO"
  echo "       Rename your mp3 clip to '$AUDIO' or update the AUDIO variable."
  exit 1
fi

echo "All input files found."
echo ""

# ---- CANVAS FILTER --------------------------------------------------------
# Background: fill the 16:9 frame and blur it. Foreground: fit the entire
# photo inside that frame. Zoom 1.0 shows the foreground whole.
CANVAS_FILTER="[0:v]split[sharp][blur];[blur]scale=${PRE_WIDTH}:${PRE_HEIGHT}:force_original_aspect_ratio=increase:flags=lanczos,crop=${PRE_WIDTH}:${PRE_HEIGHT},gblur=sigma=${BLUR_SIGMA},setsar=1[blurred];[sharp]scale=${PRE_WIDTH}:${PRE_HEIGHT}:force_original_aspect_ratio=decrease:flags=lanczos,setsar=1[fitted];[blurred][fitted]overlay=(main_w-overlay_w)/2:(main_h-overlay_h)/2"

# Pan progress for the zoom-in patterns: 0 during the opening hold, 1 at the
# end of the clip. Multiplied by (iw-iw/zoom), so it stays put while zoom is 1.
PAN_T="min(max(time-${T_HOLD_IN},0)/${TRAVEL_DUR},1)"

# ---- GENERATE INDIVIDUAL CLIPS -------------------------------------------
echo "Step 1/3: Generating Ken Burns clips..."

for i in "${!IMAGES[@]}"; do
  img="${IMAGES[$i]}"
  mod=$((i % 4))

  dims=$(ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=s=x:p=0 "$img")
  img_w=${dims%x*}
  img_h=${dims#*x}
  # Zoom that just covers the 16:9 frame. Taller photos pillarbox and need
  # more zoom to fill the width; wider photos letterbox and fill the height.
  FILL_ZOOM=$(awk -v w="$img_w" -v h="$img_h" -v pw="$PRE_WIDTH" -v ph="$PRE_HEIGHT" 'BEGIN {
    aspect = w / h
    canvas = pw / ph
    if (aspect >= canvas)
      z = aspect / canvas
    else
      z = canvas / aspect
    printf "%.4f", z
  }')
  # Far end of the move: the speed limit, or the zoom that just fills the frame,
  # whichever stays closer to the whole photo.
  MAX_ZOOM=$(awk -v fill="$FILL_ZOOM" -v limit="$ZOOM_LIMIT" -v full="$FULL_ZOOM" 'BEGIN {
    z = fill
    if (limit < z) z = limit
    if (z < full) z = full
    printf "%.4f", z
  }')

  # Four motion patterns cycled for variety. Zoom-in patterns hold the whole
  # photo through the fade-in plus HOLD_DUR. The zoom-out pattern arrives on
  # the whole photo HOLD_DUR before the fade-out and stays there.
  case $mod in
    0)
      # Zoom in, centered
      ZOOM="min(${FULL_ZOOM}+${ZOOM_SPEED}*max(time-${T_HOLD_IN},0),${MAX_ZOOM})"
      PANX="iw/2-(iw/zoom/2)"
      PANY="ih/2-(ih/zoom/2)"
      ;;
    1)
      # Zoom in, pan left to right
      ZOOM="min(${FULL_ZOOM}+${ZOOM_SPEED}*max(time-${T_HOLD_IN},0),${MAX_ZOOM})"
      PANX="(iw-iw/zoom)*(${PAN_T})"
      PANY="ih/2-(ih/zoom/2)"
      ;;
    2)
      # Zoom out, centered
      ZOOM="max(${MAX_ZOOM}-${ZOOM_SPEED}*time,${FULL_ZOOM})"
      PANX="iw/2-(iw/zoom/2)"
      PANY="ih/2-(ih/zoom/2)"
      ;;
    3)
      # Zoom in, pan top to bottom
      ZOOM="min(${FULL_ZOOM}+${ZOOM_SPEED}*max(time-${T_HOLD_IN},0),${MAX_ZOOM})"
      PANX="iw/2-(iw/zoom/2)"
      PANY="(ih-ih/zoom)*(${PAN_T})"
      ;;
  esac

  echo "  Clip $((i+1))/${NUM_IMAGES}: $img  (fill ${FILL_ZOOM}, max zoom ${MAX_ZOOM}, pattern ${mod})"

  # One still of the fitted photo. Blur runs once, not on every zoompan frame.
  ffmpeg -y -i "$img" \
    -filter_complex "${CANVAS_FILTER}" \
    -frames:v 1 -update 1 \
    "canvas_${i}.png" 2>&1 | grep -v "^$" | tail -1

  # d=1: generate 1 output frame per input frame
  # time-based zoom: zoom advances smoothly based on elapsed time, not frame count
  # -framerate sets input rate to match output fps
  ffmpeg -y -loop 1 -framerate "$FPS" -i "canvas_${i}.png" \
    -vf "zoompan=z='${ZOOM}':x='${PANX}':y='${PANY}':d=1:s=${WIDTH}x${HEIGHT}:fps=${FPS},format=yuv420p" \
    -t "$CLIP_DUR" \
    -c:v libx264 -preset medium -crf 20 \
    "clip_${i}.mp4" 2>&1 | grep -v "^$" | tail -1
done

echo ""
echo "Step 2/3: Crossfading clips together..."

# ---- BUILD XFADE CHAIN ----------------------------------------------------
# Chain: [0][1]xfade -> [x1], [x1][2]xfade -> [x2], ...
# Offset for xfade N (1-indexed) = N * (clip_dur - xfade_dur)
FILTER=""
LAST_LABEL=""
for i in $(seq 0 $((NUM_IMAGES - 2))); do
  offset=$(awk "BEGIN { printf \"%.4f\", ($i + 1) * $XFADE_STEP }")
  out_label="x$((i + 1))"
  if [ "$i" -eq 0 ]; then
    FILTER="[0:v][1:v]xfade=transition=fade:duration=${XFADE_DUR}:offset=${offset}[${out_label}]"
  else
    FILTER="${FILTER};[${LAST_LABEL}][$((i+1)):v]xfade=transition=fade:duration=${XFADE_DUR}:offset=${offset}[${out_label}]"
  fi
  LAST_LABEL="$out_label"
done

# Build input argument list
INPUT_ARGS=""
for i in "${!IMAGES[@]}"; do
  INPUT_ARGS="${INPUT_ARGS} -i clip_${i}.mp4"
done

# Expected total video duration
EXPECTED_DUR=$(awk "BEGIN { printf \"%.2f\", $NUM_IMAGES * $CLIP_DUR - ($NUM_IMAGES - 1) * $XFADE_DUR }")
echo "  Expected video duration: ${EXPECTED_DUR}s"

ffmpeg -y \
  ${INPUT_ARGS} \
  -filter_complex "${FILTER}" \
  -map "[${LAST_LABEL}]" \
  -c:v libx264 -preset medium -crf 20 \
  -pix_fmt yuv420p \
  -t "$EXPECTED_DUR" \
  "video_crossfaded.mp4" 2>&1 | grep -v "^$" | tail -3

echo ""
echo "Step 3/3: Mixing audio..."

# ---- ADD AUDIO -----------------------------------------------------------
ffmpeg -y \
  -i "video_crossfaded.mp4" \
  -i "$AUDIO" \
  -c:v copy \
  -c:a aac -b:a 192k \
  -map 0:v:0 -map 1:a:0 \
  -shortest \
  -movflags +faststart \
  "$OUTPUT" 2>&1 | grep -v "^$" | tail -3

# ---- CLEANUP -------------------------------------------------------------
echo ""
echo "Cleaning up temporary files..."
rm -f clip_*.mp4 canvas_*.png video_crossfaded.mp4

echo ""
echo "=========================================="
echo "DONE: $OUTPUT"
echo "=========================================="
FINAL_DUR=$(ffprobe -show_entries format=duration -v quiet -of csv="p=0" "$OUTPUT" 2>/dev/null || echo "unknown")
echo "Final video duration: ${FINAL_DUR}s"
