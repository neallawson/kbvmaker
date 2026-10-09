// Package media probes durations and image sizes, and checks the ffmpeg build.
package media

import "context"

// Info is the displayed size of one still, after a 90° or 270° rotation.
type Info struct {
	Width  int
	Height int
}

// Probe reads media without rendering it.
type Probe interface {
	Duration(ctx context.Context, path string) (float64, error)
	Image(ctx context.Context, path string) (Info, error)
	Filters(ctx context.Context) (map[string]bool, error)
}

// RequiredFilters are the ffmpeg filters a Montage 1 render uses.
var RequiredFilters = []string{"zoompan", "xfade", "gblur"}

// Fake is a Probe for tests.
type Fake struct {
	Durations map[string]float64
	Images    map[string]Info
	Have      map[string]bool
}

// Duration returns the canned song length.
func (f *Fake) Duration(_ context.Context, path string) (float64, error) {
	d, ok := f.Durations[path]
	if !ok {
		return 0, errMissing(path)
	}
	return d, nil
}

// Image returns the canned displayed size.
func (f *Fake) Image(_ context.Context, path string) (Info, error) {
	info, ok := f.Images[path]
	if !ok {
		return Info{}, errMissing(path)
	}
	return info, nil
}

// Filters returns the canned filter set, or the three Montage requires.
func (f *Fake) Filters(context.Context) (map[string]bool, error) {
	if f.Have != nil {
		return f.Have, nil
	}
	return map[string]bool{"zoompan": true, "xfade": true, "gblur": true}, nil
}

type missing string

func (m missing) Error() string { return "missing media " + string(m) }

func errMissing(path string) error { return missing(path) }
