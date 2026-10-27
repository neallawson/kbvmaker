package doc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"kbvmaker/v1/rule"
	"kbvmaker/v1/version"
)

// Load reads a Montage document and applies defaults.
// Paths stay as written. Dir is the directory that contains path.
func Load(path string) (*Document, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d, err := Decode(b)
	if err != nil {
		return nil, err
	}
	d.Dir = dirOf(path)
	return d, nil
}

// Decode parses a Montage document and applies defaults.
func Decode(b []byte) (*Document, error) {
	if err := rejectDup(b); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var raw rawDoc
	if err := dec.Decode(&raw); err != nil {
		return nil, wrapJSON(err)
	}
	if err := trailing(dec); err != nil {
		return nil, err
	}
	return raw.document()
}

func trailing(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return rule.E(0, "bad-number", "Bad Montage File Format: extra text follows the montage")
		}
		return wrapJSON(err)
	}
	return nil
}

func wrapJSON(err error) error {
	msg := err.Error()
	const prefix = "json: unknown field "
	if field, ok := strings.CutPrefix(msg, prefix); ok {
		name, err := strconv.Unquote(field)
		if err != nil {
			name = field
		}
		return unsupportedField(name)
	}
	if strings.Contains(msg, "unknown field") {
		return rule.E(0, "unknown-field", "Bad Montage File Format: %s", strings.TrimPrefix(msg, "json: "))
	}
	return rule.E(0, "bad-number", "Bad Montage File Format: %s", jsonProblem(msg))
}

func jsonProblem(msg string) string {
	msg = strings.TrimPrefix(msg, "json: ")
	const mark = "field "
	if strings.Contains(msg, "cannot unmarshal") && strings.Contains(msg, mark) {
		rest := msg[strings.Index(msg, mark)+len(mark):]
		if dot := strings.LastIndex(rest, "."); dot >= 0 {
			rest = rest[dot+1:]
		}
		name := rest
		if sp := strings.IndexByte(name, ' '); sp >= 0 {
			name = name[:sp]
		}
		if name != "" {
			return fmt.Sprintf("%q has the wrong kind of value", name)
		}
		return "a value has the wrong kind"
	}
	if msg == "" {
		return "the file is not valid JSON"
	}
	return "the file is not valid JSON (" + msg + ")"
}

func unsupportedField(name string) error {
	return rule.E(0, "unknown-field", "Bad Montage File Format: contains unsupported %q field", name)
}

type rawDoc struct {
	Montage  *float64     `json:"montage"`
	Canvas   *rawCanvas   `json:"canvas"`
	Output   *rawOutput   `json:"output"`
	Defaults *rawDefaults `json:"defaults"`
	Tracks   []Track      `json:"tracks"`
	Timing   *Timing      `json:"timing"`
	Pool     *Pool        `json:"pool"`
	Schedule *Schedule    `json:"schedule"`
	Modes    ModeMap      `json:"modes"`
	Moves    []string     `json:"moves"`
	Slots    []Slot       `json:"slots"`
}

type rawCanvas struct {
	Width    *float64  `json:"width"`
	Height   *float64  `json:"height"`
	Blur     *float64  `json:"blur"`
	Backdrop *Backdrop `json:"backdrop"`
}

type rawOutput struct {
	Width        *float64 `json:"width"`
	Height       *float64 `json:"height"`
	FPS          *float64 `json:"fps"`
	File         *string  `json:"file"`
	VideoCodec   *string  `json:"videoCodec"`
	CRF          *float64 `json:"crf"`
	Preset       *string  `json:"preset"`
	PixFmt       *string  `json:"pixFmt"`
	AudioCodec   *string  `json:"audioCodec"`
	AudioBitrate *float64 `json:"audioBitrate"`
}

type rawDefaults struct {
	Join     *Join    `json:"join"`
	Hold     *float64 `json:"hold"`
	ZoomSpan *float64 `json:"zoomSpan"`
	Ease     *string  `json:"ease"`
}

func (r rawDoc) document() (*Document, error) {
	if r.Montage == nil {
		return nil, rule.E(0, "missing-field", `Bad Montage File Format: the file needs "montage": 1`)
	}
	if *r.Montage != float64(version.Current) {
		return nil, rule.E(0, "bad-number", "Bad Montage File Format: version %v is not supported", *r.Montage)
	}
	d := &Document{Montage: version.Current, Tracks: r.Tracks}
	d.Canvas = Canvas{Width: 3840, Height: 2160, Backdrop: blurBackdrop(40)}
	if c := r.Canvas; c != nil {
		if c.Width != nil {
			n, err := evenPx(*c.Width, "canvas.width")
			if err != nil {
				return nil, err
			}
			d.Canvas.Width = n
		}
		if c.Height != nil {
			n, err := evenPx(*c.Height, "canvas.height")
			if err != nil {
				return nil, err
			}
			d.Canvas.Height = n
		}
		if c.Blur != nil && c.Backdrop != nil {
			return nil, rule.E(0, "unknown-field", "Bad Montage File Format: canvas sets both blur and backdrop")
		}
		if c.Backdrop != nil {
			if err := c.Backdrop.finish(0); err != nil {
				return nil, err
			}
			d.Canvas.Backdrop = *c.Backdrop
		} else if c.Blur != nil {
			if *c.Blur < 0 {
				return nil, rule.E(0, "bad-number", "blur %v must be zero or more", *c.Blur)
			}
			d.Canvas.Backdrop = blurBackdrop(*c.Blur)
		}
	}
	for i := range r.Slots {
		if r.Slots[i].Backdrop == nil {
			continue
		}
		if err := r.Slots[i].Backdrop.finish(i + 1); err != nil {
			return nil, err
		}
	}
	d.Output = Output{
		Width: 1920, Height: 1080, FPS: 30,
		VideoCodec: "libx264", CRF: 20, Preset: "medium", PixFmt: "yuv420p",
		AudioCodec: "aac", AudioBitrate: 192000,
	}
	if o := r.Output; o != nil {
		if err := applyOutput(&d.Output, o); err != nil {
			return nil, err
		}
	}
	d.Defaults = Defaults{
		Join:     Join{Type: "fade", Duration: 1.5},
		Hold:     1,
		ZoomSpan: 0.4,
		Ease:     "linear",
	}
	if df := r.Defaults; df != nil {
		if df.Join != nil {
			d.Defaults.Join = *df.Join
		}
		if df.Hold != nil {
			d.Defaults.Hold = *df.Hold
		}
		if df.ZoomSpan != nil {
			d.Defaults.ZoomSpan = *df.ZoomSpan
		}
		if df.Ease != nil {
			d.Defaults.Ease = *df.Ease
		}
	}
	d.Timing = Timing{Policy: "fit-audio"}
	if r.Timing != nil {
		d.Timing = *r.Timing
	}
	d.Pool = r.Pool
	d.Schedule = r.Schedule
	d.Modes = r.Modes
	d.Moves = r.Moves
	d.Slots = r.Slots
	if d.Canvas.Width < d.Output.Width || d.Canvas.Height < d.Output.Height {
		return nil, rule.E(0, "bad-number", "canvas %dx%d is smaller than output %dx%d",
			d.Canvas.Width, d.Canvas.Height, d.Output.Width, d.Output.Height)
	}
	return d, nil
}

func applyOutput(o *Output, raw *rawOutput) error {
	if raw.Width != nil {
		n, err := evenPx(*raw.Width, "output.width")
		if err != nil {
			return err
		}
		o.Width = n
	}
	if raw.Height != nil {
		n, err := evenPx(*raw.Height, "output.height")
		if err != nil {
			return err
		}
		o.Height = n
	}
	if raw.FPS != nil {
		n, err := positiveInt(*raw.FPS, "fps")
		if err != nil {
			return err
		}
		o.FPS = n
	}
	if raw.File != nil {
		o.File = *raw.File
	}
	if raw.VideoCodec != nil {
		o.VideoCodec = *raw.VideoCodec
	}
	if raw.CRF != nil {
		o.CRF = *raw.CRF
	}
	if raw.Preset != nil {
		o.Preset = *raw.Preset
	}
	if raw.PixFmt != nil {
		o.PixFmt = *raw.PixFmt
	}
	if raw.AudioCodec != nil {
		o.AudioCodec = *raw.AudioCodec
	}
	if raw.AudioBitrate != nil {
		n, err := positiveInt(*raw.AudioBitrate, "audioBitrate")
		if err != nil {
			return err
		}
		o.AudioBitrate = n
	}
	return nil
}

func evenPx(v float64, name string) (int, error) {
	n, err := positiveInt(v, name)
	if err != nil {
		return 0, err
	}
	if n%2 != 0 {
		return 0, rule.E(0, "bad-number", "%s %d must be an even number of pixels", name, n)
	}
	return n, nil
}

func positiveInt(v float64, name string) (int, error) {
	if v <= 0 || v != float64(int(v)) || v > 1<<31-1 {
		return 0, rule.E(0, "bad-number", "%s %v must be a positive whole number", name, v)
	}
	return int(v), nil
}

func dirOf(path string) string {
	i := strings.LastIndex(path, string(os.PathSeparator))
	if i < 0 {
		return "."
	}
	if i == 0 {
		return string(os.PathSeparator)
	}
	return path[:i]
}

// Override is a flag that wins over the document.
type Override struct {
	Audio      string
	OutputFile string
	Join       *float64
	Hold       *float64
	ZoomSpan   *float64
	Blur       *float64
	Seed       *uint64
	Schedule   string
}

// ApplyOverrides writes flag values onto a decoded document.
func ApplyOverrides(d *Document, o Override) {
	if o.Audio != "" {
		d.Tracks = []Track{{Src: o.Audio}}
	}
	if o.OutputFile != "" {
		d.Output.File = o.OutputFile
	}
	if o.Join != nil {
		d.Defaults.Join.Duration = *o.Join
	}
	if o.Hold != nil {
		d.Defaults.Hold = *o.Hold
	}
	if o.ZoomSpan != nil {
		d.Defaults.ZoomSpan = *o.ZoomSpan
	}
	if o.Blur != nil {
		switch d.Canvas.Backdrop.Kind {
		case "blur", "image":
			v := *o.Blur
			d.Canvas.Backdrop.Blur = &v
		}
	}
	if o.Schedule != "" {
		if d.Schedule == nil {
			d.Schedule = &Schedule{}
		}
		d.Schedule.Policy = o.Schedule
	}
	if o.Seed != nil {
		if d.Schedule == nil {
			d.Schedule = &Schedule{Policy: "shuffle"}
		}
		d.Schedule.Seed = *o.Seed
		d.Schedule.HasSeed = true
	}
}
