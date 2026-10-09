package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// FFProbe reads durations and image sizes with ffprobe, and lists ffmpeg filters.
type FFProbe struct {
	Probe  string
	FFmpeg string
}

// NewFFProbe uses the ffprobe and ffmpeg binaries on PATH.
func NewFFProbe() (*FFProbe, error) {
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, err
	}
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, err
	}
	return &FFProbe{Probe: probe, FFmpeg: ff}, nil
}

// Duration is the container duration in seconds.
func (f *FFProbe) Duration(ctx context.Context, path string) (float64, error) {
	out, err := run(ctx, f.Probe, "-v", "error", "-show_entries", "format=duration", "-of", "json", path)
	if err != nil {
		return 0, err
	}
	var parsed struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return 0, err
	}
	if parsed.Format.Duration == "" {
		return 0, fmt.Errorf("no duration in %s", path)
	}
	return strconv.ParseFloat(parsed.Format.Duration, 64)
}

// Image is the displayed size. A 90° or 270° rotation swaps the coded size.
func (f *FFProbe) Image(ctx context.Context, path string) (Info, error) {
	out, err := run(ctx, f.Probe, "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height:stream_side_data=rotation:stream_tags=rotate",
		"-of", "json", path)
	if err != nil {
		return Info{}, err
	}
	return parseImage(out)
}

func parseImage(out []byte) (Info, error) {
	var parsed struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
			Tags   struct {
				Rotate string `json:"rotate"`
			} `json:"tags"`
			SideData []struct {
				Rotation float64 `json:"rotation"`
			} `json:"side_data_list"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return Info{}, err
	}
	if len(parsed.Streams) == 0 || parsed.Streams[0].Width == 0 {
		return Info{}, fmt.Errorf("no video stream")
	}
	s := parsed.Streams[0]
	rot := 0.0
	if len(s.SideData) > 0 {
		rot = s.SideData[0].Rotation
	} else if s.Tags.Rotate != "" {
		rot, _ = strconv.ParseFloat(s.Tags.Rotate, 64)
	}
	w, h := s.Width, s.Height
	if turned(rot) {
		w, h = h, w
	}
	return Info{Width: w, Height: h}, nil
}

func turned(rot float64) bool {
	r := rot
	if r < 0 {
		r = -r
	}
	d := r - 90
	if d < 0 {
		d = -d
	}
	if d < 1 {
		return true
	}
	d = r - 270
	if d < 0 {
		d = -d
	}
	return d < 1
}

// Filters reports which ffmpeg filter names are available.
func (f *FFProbe) Filters(ctx context.Context) (map[string]bool, error) {
	out, err := run(ctx, f.FFmpeg, "-hide_banner", "-filters")
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "-") || fields[0] == "Filters:" {
			continue
		}
		// " T. zoompan  V->V  ..." or " .. zoompan V->V"
		name := fields[0]
		if strings.Contains(name, ".") || strings.Contains(name, ">") || name == "T" || name == "TS" {
			if len(fields) > 1 {
				name = fields[1]
			}
		}
		have[name] = true
	}
	return have, nil
}

func run(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", bin, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
