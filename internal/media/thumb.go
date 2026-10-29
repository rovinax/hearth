package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hearth/internal/store"
)

func MakeThumb(ctx context.Context, bins Binaries, original, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	partial := strings.TrimSuffix(dest, filepath.Ext(dest)) + ".partial.jpg"
	defer os.Remove(partial)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var cmd *exec.Cmd
	switch {
	case bins.Vips != "":
		cmd = exec.CommandContext(ctx, bins.Vips, "thumbnail", original, partial, "480")
	case bins.FFmpeg != "":
		cmd = exec.CommandContext(ctx, bins.FFmpeg, "-y", "-i", original, "-vf", "scale=480:-1", "-frames:v", "1", partial)
	default:
		return errors.New("no thumbnail tool")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 300 {
			msg = msg[:300]
		}
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return os.Rename(partial, dest)
}

func ThumbJob(st *store.Store, originals, cache string, bins Binaries) func(context.Context, string) error {
	return func(ctx context.Context, payload string) error {
		id, err := strconv.ParseInt(payload, 10, 64)
		if err != nil {
			return err
		}
		file, err := st.GetFile(ctx, id)
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if file.MediaKind != store.KindImage {
			return nil
		}
		src, err := safeJoin(originals, file.RelPath)
		if err != nil {
			return err
		}
		rel := "thumbs/" + file.SHA256[:2] + "/" + file.SHA256[2:4] + "/" + file.SHA256 + ".jpg"
		dest, err := safeJoin(cache, rel)
		if err != nil {
			return err
		}
		if err := MakeThumb(ctx, bins, src, dest); err != nil {
			return err
		}
		return st.SetThumb(ctx, file.ID, rel)
	}
}

func safeJoin(root, rel string) (string, error) {
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("bad path")
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}
