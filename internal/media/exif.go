package media

import (
	"os"

	"github.com/rwcarlsen/goexif/exif"
)

func TakenAt(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	meta, err := exif.Decode(f)
	if err != nil {
		return ""
	}
	when, err := meta.DateTime()
	if err != nil || when.IsZero() {
		return ""
	}
	return when.UTC().Format("2006-01-02T15:04:05.000Z")
}
