package main

import (
	"context"
	"path/filepath"
)

// Dialogs opens the native file dialogs.
// Each dialog remembers the folder of the last file it returned.
type Dialogs struct {
	ctx        context.Context
	montageDir string
	imageDir   string
	audioDir   string
	scriptDir  string
	videoDir   string
}

func (d *Dialogs) remember(path string, dir *string) {
	if path == "" {
		return
	}
	*dir = filepath.Dir(path)
}

func (d *Dialogs) startup(ctx context.Context) {
	d.ctx = ctx
}

// OpenMontage asks for a project file and returns its path.
// An empty path means the dialog was cancelled.
func (d *Dialogs) OpenMontage() (string, error) {
	paths, err := d.pickFiles("open", "Open Montage", d.montageDir, "", "Montage", []string{"*.montage.json", "*.json"})
	if err != nil {
		return "", err
	}
	path := firstPath(paths)
	d.remember(path, &d.montageDir)
	return path, nil
}

// SaveMontage asks where a new project should be written.
func (d *Dialogs) SaveMontage() (string, error) {
	paths, err := d.pickFiles("save", "Save Montage", d.montageDir, "piece.montage.json", "Montage", []string{"*.montage.json", "*.json"})
	if err != nil {
		return "", err
	}
	path := montagePath(firstPath(paths))
	d.remember(path, &d.montageDir)
	return path, nil
}

// OpenImages asks for slideshow pictures.
func (d *Dialogs) OpenImages() ([]string, error) {
	paths, err := d.pickFiles("multi", "Add pictures", d.imageDir, "", "Images", []string{"*.jpg", "*.jpeg", "*.png", "*.webp", "*.JPG", "*.JPEG", "*.PNG", "*.WEBP"})
	if err != nil {
		return nil, err
	}
	if len(paths) > 0 {
		d.remember(paths[0], &d.imageDir)
	}
	return paths, nil
}

// OpenAudio asks for the soundtrack.
func (d *Dialogs) OpenAudio() (string, error) {
	paths, err := d.pickFiles("open", "Add song", d.audioDir, "", "Audio", []string{"*.mp3", "*.wav", "*.m4a", "*.flac", "*.aac", "*.MP3", "*.WAV", "*.M4A", "*.FLAC", "*.AAC"})
	if err != nil {
		return "", err
	}
	path := firstPath(paths)
	d.remember(path, &d.audioDir)
	return path, nil
}

// SaveScript asks where the shell script should be written.
func (d *Dialogs) SaveScript() (string, error) {
	paths, err := d.pickFiles("save", "Export shell script", d.scriptDir, "render.sh", "Shell script", []string{"*.sh"})
	if err != nil {
		return "", err
	}
	path := withExt(firstPath(paths), ".sh")
	d.remember(path, &d.scriptDir)
	return path, nil
}

// SaveVideo asks where the rendered video should be written.
func (d *Dialogs) SaveVideo() (string, error) {
	paths, err := d.pickFiles("save", "Render video", d.videoDir, "out.mp4", "Video", []string{"*.mp4"})
	if err != nil {
		return "", err
	}
	path := withExt(firstPath(paths), ".mp4")
	d.remember(path, &d.videoDir)
	return path, nil
}
