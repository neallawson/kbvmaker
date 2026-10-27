// Package schedule turns a pool of pictures into an ordered slot list.
package schedule

import (
	"math"
	"os"
	"path/filepath"
	"strings"

	"kbvmaker/v1/doc"
	"kbvmaker/v1/rule"
)

// RNG is the splitmix64 generator from the Montage v1 specification.
type RNG struct {
	state uint64
}

// NewRNG starts at seed. The first Next call mixes seed before returning.
func NewRNG(seed uint64) *RNG { return &RNG{state: seed} }

// Next returns the next uint64.
func (r *RNG) Next() uint64 {
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// Intn returns a uniform integer in [0, n). n must be positive.
func (r *RNG) Intn(n int) int {
	if n <= 1 {
		return 0
	}
	un := uint64(n)
	// Powers of two divide 2^64, so every sample is usable.
	if un&(un-1) == 0 {
		return int(r.Next() & (un - 1))
	}
	rem := (math.MaxUint64 % un) + 1 // 2^64 % n
	limit := math.MaxUint64 - (rem - 1)
	for {
		x := r.Next()
		if x < limit {
			return int(x % un)
		}
	}
}

// NaturalLess compares paths with digit runs as integers.
// Equal integers use the shorter digit run first.
func NaturalLess(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if isDigit(a[i]) && isDigit(b[j]) {
			ie := i
			for ie < len(a) && isDigit(a[ie]) {
				ie++
			}
			je := j
			for je < len(b) && isDigit(b[je]) {
				je++
			}
			as, bs := a[i:ie], b[j:je]
			if c := cmpInt(as, bs); c != 0 {
				return c < 0
			}
			if len(as) != len(bs) {
				return len(as) < len(bs)
			}
			i, j = ie, je
			continue
		}
		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
	return len(a) < len(b)
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func cmpInt(a, b string) int {
	aa := strings.TrimLeft(a, "0")
	bb := strings.TrimLeft(b, "0")
	if aa == "" {
		aa = "0"
	}
	if bb == "" {
		bb = "0"
	}
	if len(aa) != len(bb) {
		if len(aa) < len(bb) {
			return -1
		}
		return 1
	}
	if aa < bb {
		return -1
	}
	if aa > bb {
		return 1
	}
	return 0
}

var imageExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".tif": true,
	".tiff": true, ".webp": true, ".bmp": true,
}

// Generate reads the pool and returns slots in schedule order.
// song is the soundtrack duration, used when a loop count falls out of fixed-clip.
func Generate(d *doc.Document, song float64) ([]doc.Slot, error) {
	files, err := poolFiles(d)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, rule.E(0, "pool", "the picture pool is empty")
	}
	n, err := count(d, song, len(files))
	if err != nil {
		return nil, err
	}
	ordered, err := order(files, d.Schedule.Policy, d.Schedule.Seed, n)
	if err != nil {
		return nil, err
	}
	slots := make([]doc.Slot, len(ordered))
	for i, src := range ordered {
		slots[i] = doc.Slot{Src: src, Mode: "in-center"}
	}
	return slots, nil
}

func count(d *doc.Document, song float64, poolSize int) (int, error) {
	switch d.Schedule.Policy {
	case "given", "shuffle":
		if d.Timing.Policy == "fixed-clip" {
			n, err := fixedCount(song, d.Timing.Clip, d.Defaults.Join.Duration)
			if err != nil {
				return 0, err
			}
			if n != poolSize {
				return 0, rule.E(0, "pool", "pool has %d pictures and fixed-clip needs %d", poolSize, n)
			}
		}
		return poolSize, nil
	case "loop":
		if poolSize < 2 {
			return 0, rule.E(0, "pool", "loop needs at least two pictures")
		}
		switch d.Timing.Policy {
		case "fixed-clip":
			return fixedCount(song, d.Timing.Clip, d.Defaults.Join.Duration)
		case "fit-audio":
			if !d.Timing.HasCount || d.Timing.Count < 1 {
				return 0, rule.E(0, "missing-field", "a loop on the soundtrack clock needs timing.count")
			}
			return d.Timing.Count, nil
		default:
			return 0, rule.E(0, "pool", "%s cannot take its pictures from a loop", d.Timing.Policy)
		}
	default:
		return 0, rule.E(0, "unknown-field", "schedule policy %q is not supported", d.Schedule.Policy)
	}
}

func fixedCount(song, clip, join float64) (int, error) {
	if clip <= join {
		return 0, rule.E(0, "bad-number", "each picture is %v seconds, which is not longer than the %v second dissolve", clip, join)
	}
	n := int(math.Round((song - join) / (clip - join)))
	if n < 1 {
		n = 1
	}
	return n, nil
}

func order(files []string, policy string, seed uint64, n int) ([]string, error) {
	switch policy {
	case "given":
		return append([]string(nil), files...), nil
	case "shuffle":
		out := append([]string(nil), files...)
		r := NewRNG(seed)
		for i := len(out) - 1; i >= 1; i-- {
			j := r.Intn(i + 1)
			out[i], out[j] = out[j], out[i]
		}
		return out, nil
	case "loop":
		r := NewRNG(seed)
		out := make([]string, n)
		prev := -1
		for i := 0; i < n; i++ {
			var idx int
			if prev < 0 {
				idx = r.Intn(len(files))
			} else {
				idx = r.Intn(len(files) - 1)
				if idx >= prev {
					idx++
				}
			}
			prev = idx
			out[i] = files[idx]
		}
		return out, nil
	default:
		return nil, rule.E(0, "unknown-field", "schedule policy %q is not supported", policy)
	}
}

func poolFiles(d *doc.Document) ([]string, error) {
	switch d.Pool.Kind {
	case "files":
		return append([]string(nil), d.Pool.Files...), nil
	case "dir":
		dir := doc.ResolvePath(d.Dir, d.Pool.Dir)
		names, err := listDir(dir, d.Pool.Sort)
		if err != nil {
			return nil, err
		}
		out := make([]string, len(names))
		for i, name := range names {
			out[i] = filepath.ToSlash(filepath.Join(d.Pool.Dir, name))
		}
		return out, nil
	case "list":
		return readList(d.Dir, doc.ResolvePath(d.Dir, d.Pool.List))
	default:
		return nil, rule.E(0, "pool", "the picture pool is empty")
	}
}

func listDir(dir, sortName string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, rule.E(0, "missing-media", "%s", err.Error())
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !imageExt[strings.ToLower(filepath.Ext(name))] {
			continue
		}
		names = append(names, name)
	}
	if sortName == "lexical" {
		sortStrings(names, func(a, b string) bool { return a < b })
	} else {
		sortStrings(names, NaturalLess)
	}
	return names, nil
}

func sortStrings(a []string, less func(string, string) bool) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && less(a[j], a[j-1]); j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func readList(projectDir, listPath string) ([]string, error) {
	b, err := os.ReadFile(listPath)
	if err != nil {
		return nil, rule.E(0, "missing-media", "%s", err.Error())
	}
	listDir := filepath.Dir(listPath)
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		abs := line
		if !filepath.IsAbs(line) {
			abs = filepath.Join(listDir, line)
		}
		out = append(out, relProject(projectDir, abs))
	}
	return out, nil
}

func relProject(projectDir, abs string) string {
	rel, err := filepath.Rel(projectDir, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return abs
	}
	return filepath.ToSlash(rel)
}
