package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

const (
	// ThumbMax is the largest edge, in pixels, a thumbnail request may ask for.
	ThumbMax      = 512
	thumbCacheMax = 128
)

type thumbHit struct {
	key  string
	jpeg []byte
}

var thumbs = struct {
	sync.Mutex
	hits []thumbHit
}{}

// Thumbnail returns a JPEG of exactly width by height.
// The picture is scaled to cover that box and center-cropped.
// A repeated request for an unchanged file is served from memory.
func Thumbnail(ctx context.Context, path string, width, height int) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s\x00%d\x00%d\x00%d", path, info.ModTime().UnixNano(), width, height)
	if jpeg, ok := thumbGet(key); ok {
		return jpeg, nil
	}
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, err
	}
	vf := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", width, height, width, height)
	jpeg, err := run(ctx, ff,
		"-hide_banner", "-loglevel", "error",
		"-i", path,
		"-vf", vf,
		"-frames:v", "1",
		"-an",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", "5",
		"pipe:1",
	)
	if err != nil {
		return nil, err
	}
	if len(jpeg) == 0 {
		return nil, fmt.Errorf("ffmpeg wrote no thumbnail for %s", path)
	}
	thumbPut(key, jpeg)
	return jpeg, nil
}

func thumbGet(key string) ([]byte, bool) {
	thumbs.Lock()
	defer thumbs.Unlock()
	for _, hit := range thumbs.hits {
		if hit.key == key {
			return hit.jpeg, true
		}
	}
	return nil, false
}

func thumbPut(key string, jpeg []byte) {
	thumbs.Lock()
	defer thumbs.Unlock()
	if len(thumbs.hits) >= thumbCacheMax {
		thumbs.hits = thumbs.hits[1:]
	}
	thumbs.hits = append(thumbs.hits, thumbHit{key: key, jpeg: jpeg})
}
