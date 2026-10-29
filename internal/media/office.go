package media

import (
	"archive/zip"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"hearth/internal/store"
)

// OfficeThumb 取出 Office 包里自带的封面图。
// PowerPoint / Word / Excel 通常写在 docProps/thumbnail.jpeg，OpenDocument 写在 Thumbnails/thumbnail.png。
func OfficeThumb(path string) ([]byte, string, bool) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, "", false
	}
	defer reader.Close()
	for _, file := range reader.File {
		name := strings.ToLower(strings.ReplaceAll(file.Name, "\\", "/"))
		switch name {
		case "docprops/thumbnail.jpeg", "docprops/thumbnail.jpg":
			data, err := readZipFile(file)
			if err != nil || !isJPEG(data) {
				return nil, "", false
			}
			return data, ".jpg", true
		case "thumbnails/thumbnail.png":
			data, err := readZipFile(file)
			if err != nil || !isPNG(data) {
				return nil, "", false
			}
			return data, ".png", true
		}
	}
	return nil, "", false
}

func IsOfficePackage(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pptx", ".pptm", ".ppsx", ".potx", ".docx", ".docm", ".xlsx", ".xlsm", ".odp", ".odt", ".ods":
		return true
	default:
		return false
	}
}

func SaveOfficeThumb(original, cache, sum string) (string, error) {
	if len(sum) < 4 {
		return "", nil
	}
	data, ext, ok := OfficeThumb(original)
	if !ok {
		return "", nil
	}
	rel := "thumbs/" + sum[:2] + "/" + sum[2:4] + "/" + sum + ext
	dest, err := safeJoin(cache, rel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	partial := dest + ".partial"
	if err := os.WriteFile(partial, data, 0o644); err != nil {
		os.Remove(partial)
		return "", err
	}
	if err := os.Rename(partial, dest); err != nil {
		os.Remove(partial)
		return "", err
	}
	return rel, nil
}

func BackfillOfficeThumbs(ctx context.Context, st *store.Store, originals, cache string, logger *slog.Logger) error {
	files, err := st.ListStoredFiles(ctx)
	if err != nil {
		return err
	}
	var saved int
	for _, file := range files {
		if file.ThumbRel != "" || file.DeletedAt != "" || !IsOfficePackage(file.Name) {
			continue
		}
		src, err := safeJoin(originals, file.RelPath)
		if err != nil {
			continue
		}
		rel, err := SaveOfficeThumb(src, cache, file.SHA256)
		if err != nil {
			logger.Error("office thumb", "file", file.ID, "err", err)
			continue
		}
		if rel == "" {
			continue
		}
		if err := st.SetThumb(ctx, file.ID, rel); err != nil {
			logger.Error("office thumb save", "file", file.ID, "err", err)
			continue
		}
		saved++
	}
	if saved > 0 {
		logger.Info("office thumbs", "saved", saved)
	}
	return nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	if file.UncompressedSize64 > 2<<20 {
		return nil, io.ErrUnexpectedEOF
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 2<<20+1))
}

func isJPEG(data []byte) bool {
	return len(data) > 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff
}

func isPNG(data []byte) bool {
	return len(data) > 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G'
}
