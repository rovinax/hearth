package media_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"hearth/internal/media"
)

func TestOfficeThumbFromPackage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.pptx")
	writeZip(t, path, map[string][]byte{
		"docProps/thumbnail.jpeg": {0xff, 0xd8, 0xff, 0xd9},
		"[Content_Types].xml":     []byte("<Types/>"),
	})
	data, ext, ok := media.OfficeThumb(path)
	if !ok || ext != ".jpg" || !bytesEqual(data, []byte{0xff, 0xd8, 0xff, 0xd9}) {
		t.Fatalf("thumb: ok=%v ext=%s data=%v", ok, ext, data)
	}
	cache := filepath.Join(dir, "cache")
	sum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	rel, err := media.SaveOfficeThumb(path, cache, sum)
	if err != nil {
		t.Fatal(err)
	}
	if rel == "" {
		t.Fatal("missing rel")
	}
	got, err := os.ReadFile(filepath.Join(cache, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytesEqual(got, []byte{0xff, 0xd8, 0xff, 0xd9}) {
		t.Fatalf("saved %v", got)
	}
}

func TestOfficeThumbMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := media.OfficeThumb(path); ok {
		t.Fatal("plain file looked like an office package")
	}
}

func writeZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
