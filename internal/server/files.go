package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"hearth/internal/media"
	"hearth/internal/store"
)

var (
	blobPathRE  = regexp.MustCompile(`^[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{64}$`)
	thumbPathRE = regexp.MustCompile(`^thumbs/[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{64}\.(jpg|png)$`)
)

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file, err := s.store.OpenFile(r.Context(), id, user)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "文件不可用", http.StatusInternalServerError)
		return
	}
	path, err := blobDiskPath(s.cfg.OriginalsDir(), file.RelPath)
	if err != nil {
		http.Error(w, "文件不可用", http.StatusInternalServerError)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "文件不可用", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "文件不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", file.Mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", contentDisposition(file.MediaKind, file.Name))
	http.ServeContent(w, r, file.Name, stat.ModTime(), f)
}

var (
	errTooLarge = errors.New("too large")
	errNoFile   = errors.New("no file")
	errCSRF     = errors.New("csrf")
)

func (s *Server) saveUpload(r *http.Request, user store.User, csrf string) (store.File, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, s.cfg.Upload.MaxBytes+1<<20)
	reader, err := r.MultipartReader()
	if err != nil {
		return store.File{}, err
	}
	headerOK := csrfOK(r.Header.Get("X-CSRF-Token"), csrf)
	var (
		gotCSRF      = headerOK
		sent         = csrf
		vis          = store.VisPrivate
		collectionID int64
		tmp          *os.File
		hash         = sha256.New()
		head         = make([]byte, 0, 512)
		size         int64
		name         string
	)
	defer func() {
		if tmp != nil {
			name := tmp.Name()
			tmp.Close()
			os.Remove(name)
		}
	}()
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var max *http.MaxBytesError
			if errors.As(err, &max) {
				return store.File{}, errTooLarge
			}
			return store.File{}, err
		}
		switch part.FormName() {
		case "csrf":
			buf, err := io.ReadAll(io.LimitReader(part, 128))
			part.Close()
			if err != nil {
				return store.File{}, err
			}
			sent = string(buf)
			gotCSRF = true
			if !csrfOK(sent, csrf) {
				return store.File{}, errCSRF
			}
		case "visibility":
			buf, err := io.ReadAll(io.LimitReader(part, 32))
			part.Close()
			if err != nil {
				return store.File{}, err
			}
			if v := string(buf); v == store.VisShared || v == store.VisPrivate {
				vis = v
			}
		case "collection":
			buf, err := io.ReadAll(io.LimitReader(part, 32))
			part.Close()
			if err != nil {
				return store.File{}, err
			}
			raw := strings.TrimSpace(string(buf))
			if raw == "" {
				break
			}
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id <= 0 {
				return store.File{}, store.ErrInvalid
			}
			collectionID = id
		case "file":
			if !gotCSRF || !csrfOK(sent, csrf) {
				part.Close()
				return store.File{}, errCSRF
			}
			name = sanitizeName(part.FileName())
			dir := filepath.Join(s.cfg.OriginalsDir(), ".tmp")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				part.Close()
				return store.File{}, err
			}
			tmp, err = os.CreateTemp(dir, "upload-*")
			if err != nil {
				part.Close()
				return store.File{}, err
			}
			buf := make([]byte, 32*1024)
			for {
				n, rerr := part.Read(buf)
				if n > 0 {
					if len(head) < 512 {
						need := 512 - len(head)
						if need > n {
							need = n
						}
						head = append(head, buf[:need]...)
					}
					if _, werr := hash.Write(buf[:n]); werr != nil {
						part.Close()
						return store.File{}, werr
					}
					if _, werr := tmp.Write(buf[:n]); werr != nil {
						part.Close()
						return store.File{}, werr
					}
					size += int64(n)
					if size > s.cfg.Upload.MaxBytes {
						part.Close()
						return store.File{}, errTooLarge
					}
				}
				if errors.Is(rerr, io.EOF) {
					break
				}
				if rerr != nil {
					part.Close()
					var max *http.MaxBytesError
					if errors.As(rerr, &max) {
						return store.File{}, errTooLarge
					}
					return store.File{}, rerr
				}
			}
			part.Close()
		default:
			part.Close()
		}
	}
	if tmp == nil {
		return store.File{}, errNoFile
	}
	if err := tmp.Close(); err != nil {
		return store.File{}, err
	}
	tmpName := tmp.Name()
	tmp = nil
	sum := hex.EncodeToString(hash.Sum(nil))
	rel := sum[:2] + "/" + sum[2:4] + "/" + sum
	dest, err := blobDiskPath(s.cfg.OriginalsDir(), rel)
	if err != nil {
		os.Remove(tmpName)
		return store.File{}, err
	}
	created := false
	if _, err := os.Stat(dest); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			os.Remove(tmpName)
			return store.File{}, err
		}
		if err := os.Rename(tmpName, dest); err != nil {
			if _, statErr := os.Stat(dest); statErr != nil {
				os.Remove(tmpName)
				return store.File{}, err
			}
			os.Remove(tmpName)
		} else {
			created = true
		}
	} else if err != nil {
		os.Remove(tmpName)
		return store.File{}, err
	} else {
		os.Remove(tmpName)
	}
	mimeType, kind := classifyFile(name, head, dest)
	taken := ""
	if kind == store.KindImage {
		taken = media.TakenAt(dest)
	}
	file, err := s.store.AddFile(r.Context(), store.NewFile{
		OwnerID:      user.ID,
		SHA256:       sum,
		Size:         size,
		RelPath:      rel,
		Name:         name,
		MediaKind:    kind,
		Mime:         mimeType,
		Visibility:   vis,
		TakenAt:      taken,
		CollectionID: collectionID,
	})
	if err != nil && created {
		os.Remove(dest)
		return store.File{}, err
	}
	if err == nil && kind == store.KindImage && (s.bins.Vips != "" || s.bins.FFmpeg != "") {
		if _, enqErr := s.store.Enqueue(r.Context(), "thumbnail", strconv.FormatInt(file.ID, 10), 3); enqErr != nil {
			s.log.Error("enqueue thumbnail", "err", enqErr)
		}
	}
	if err == nil && media.IsOfficePackage(name) {
		relThumb, thumbErr := media.SaveOfficeThumb(dest, s.cfg.CacheDir(), sum)
		if thumbErr != nil {
			s.log.Error("office thumb", "err", thumbErr)
		} else if relThumb != "" {
			if setErr := s.store.SetThumb(r.Context(), file.ID, relThumb); setErr != nil {
				s.log.Error("office thumb save", "err", setErr)
			} else {
				file.ThumbRel = relThumb
			}
		}
	}
	return file, err
}

func (s *Server) trashLoop(ctx context.Context) {
	s.sweepTrash(ctx, time.Now())
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.sweepTrash(ctx, now)
		}
	}
}

func (s *Server) sweepTrash(ctx context.Context, now time.Time) {
	removed, err := s.store.PurgeExpired(ctx, now)
	if err != nil {
		s.log.Error("purge trash", "err", err)
		return
	}
	s.dropRemoved(removed)
}

func (s *Server) dropRemoved(removed []store.RemovedFile) {
	for _, item := range removed {
		s.removeStored(s.cfg.OriginalsDir(), item.Rel, blobDiskPath)
		s.removeStored(s.cfg.CacheDir(), item.Thumb, thumbDiskPath)
	}
}

func (s *Server) removeStored(root, rel string, clean func(string, string) (string, error)) {
	if rel == "" {
		return
	}
	path, err := clean(root, rel)
	if err != nil {
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.log.Error("remove file", "path", path, "err", err)
	}
}

func blobDiskPath(root, rel string) (string, error) {
	if !blobPathRE.MatchString(rel) {
		return "", errors.New("bad blob path")
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

func thumbDiskPath(root, rel string) (string, error) {
	if !thumbPathRE.MatchString(rel) {
		return "", errors.New("bad thumb path")
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

func sanitizeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == '"' {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	if utf8.RuneCountInString(name) > 200 {
		runes := []rune(name)
		name = string(runes[:200])
	}
	return name
}

func classifyFile(name string, head []byte, path string) (string, string) {
	f, err := os.Open(path)
	if err != nil {
		return media.Classify(name, head, nil)
	}
	defer f.Close()
	return media.Classify(name, head, f)
}

func contentDisposition(kind, name string) string {
	disp := "attachment"
	if kind == store.KindImage || kind == store.KindVideo || kind == store.KindAudio {
		disp = "inline"
	}
	return mime.FormatMediaType(disp, map[string]string{"filename": name})
}

func cleanKind(kind string) string {
	switch kind {
	case store.KindImage, store.KindVideo, store.KindAudio, store.KindOther:
		return kind
	default:
		return ""
	}
}
