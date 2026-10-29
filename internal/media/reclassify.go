package media

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"hearth/internal/store"
)

// Reclassify 把已经入库、但被标成视频或其他的纯音频改回音频。
func Reclassify(ctx context.Context, st *store.Store, originalsDir string, logger *slog.Logger) error {
	files, err := st.ListStoredFiles(ctx)
	if err != nil {
		return err
	}
	updated := 0
	for _, file := range files {
		if file.MediaKind == store.KindImage {
			continue
		}
		path, err := storedBlob(originalsDir, file.RelPath)
		if err != nil {
			logger.Warn("reclassify path", "id", file.ID, "err", err)
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			logger.Warn("reclassify open", "id", file.ID, "err", err)
			continue
		}
		head := make([]byte, 512)
		n, _ := io.ReadFull(f, head)
		mimeType, kind := Classify(file.Name, head[:n], f)
		f.Close()
		if kind == file.MediaKind && mimeType == file.Mime {
			continue
		}
		if err := st.SetMedia(ctx, file.ID, kind, mimeType); err != nil {
			return err
		}
		updated++
		logger.Info("reclassify file", "id", file.ID, "name", file.Name, "from", file.MediaKind, "to", kind)
	}
	if updated > 0 {
		logger.Info("reclassify media", "updated", updated)
	}
	return nil
}

func storedBlob(root, rel string) (string, error) {
	clean := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	root = filepath.Clean(root)
	if clean != root && !strings.HasPrefix(clean, root+string(os.PathSeparator)) {
		return "", os.ErrInvalid
	}
	return clean, nil
}
