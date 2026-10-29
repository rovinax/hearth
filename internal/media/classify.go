package media

import (
	"encoding/binary"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"hearth/internal/store"
)

// Classify 根据文件名、文件头，以及容器里的轨道判断类型。
// 只有音轨的 mp4 / m4a / webm 归为音频，避免被视频播放器打开。
func Classify(name string, head []byte, rs io.ReadSeeker) (string, string) {
	ext := strings.ToLower(filepath.Ext(name))
	if mimeType, ok := imageMIME[ext]; ok {
		return mimeType, store.KindImage
	}

	var probeVideo, probeAudio, probed bool
	if rs != nil && shouldProbe(ext, head) {
		probeVideo, probeAudio, probed = probeContainer(ext, rs)
	}
	if probed && probeVideo {
		return videoMIME(ext), store.KindVideo
	}
	if isAudioExt(ext) || (probed && probeAudio) || ftypAudio(head) {
		return audioMIME(ext), store.KindAudio
	}
	if mimeType, ok := videoMIMEExt[ext]; ok {
		return mimeType, store.KindVideo
	}

	detected := http.DetectContentType(head)
	switch {
	case strings.HasPrefix(detected, "image/"):
		return detected, store.KindImage
	case strings.HasPrefix(detected, "audio/") || detected == "application/ogg" && isAudioExt(ext):
		if detected == "application/ogg" || detected == "" {
			return audioMIME(ext), store.KindAudio
		}
		return detected, store.KindAudio
	case strings.HasPrefix(detected, "video/"):
		if isAudioExt(ext) || ftypAudio(head) {
			return audioMIME(ext), store.KindAudio
		}
		return detected, store.KindVideo
	default:
		if isAudioExt(ext) {
			return audioMIME(ext), store.KindAudio
		}
		if detected == "" {
			detected = "application/octet-stream"
		}
		return detected, store.KindOther
	}
}

var imageMIME = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".heic": "image/heic",
	".heif": "image/heic",
}

var videoMIMEExt = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".mov":  "video/quicktime",
	".webm": "video/webm",
	".mkv":  "video/x-matroska",
	".ogv":  "video/ogg",
	".avi":  "video/x-msvideo",
	".mpeg": "video/mpeg",
	".mpg":  "video/mpeg",
	".3gp":  "video/3gpp",
	".3gpp": "video/3gpp",
	".3g2":  "video/3gpp2",
}

func isAudioExt(ext string) bool {
	switch ext {
	case ".mp3", ".wav", ".wave", ".flac", ".aac", ".m4a", ".m4b", ".ogg", ".oga", ".opus",
		".wma", ".aif", ".aiff", ".amr", ".caf", ".mid", ".midi":
		return true
	default:
		return false
	}
}

func videoMIME(ext string) string {
	if mimeType, ok := videoMIMEExt[ext]; ok {
		return mimeType
	}
	return "video/mp4"
}

func audioMIME(ext string) string {
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".wav", ".wave":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".aac":
		return "audio/aac"
	case ".ogg", ".oga", ".opus", ".ogv":
		return "audio/ogg"
	case ".wma":
		return "audio/x-ms-wma"
	case ".aif", ".aiff":
		return "audio/aiff"
	case ".amr":
		return "audio/amr"
	case ".caf":
		return "audio/x-caf"
	case ".mid", ".midi":
		return "audio/midi"
	case ".webm", ".mkv":
		return "audio/webm"
	default:
		return "audio/mp4"
	}
}

func shouldProbe(ext string, head []byte) bool {
	switch ext {
	case ".mp4", ".m4v", ".m4a", ".m4b", ".mov", ".3gp", ".3gpp", ".3g2", ".webm", ".mkv":
		return true
	}
	if len(head) >= 8 && string(head[4:8]) == "ftyp" {
		return true
	}
	return len(head) >= 4 && head[0] == 0x1a && head[1] == 0x45 && head[2] == 0xdf && head[3] == 0xa3
}

func ftypAudio(head []byte) bool {
	if len(head) < 12 || string(head[4:8]) != "ftyp" {
		return false
	}
	switch string(head[8:12]) {
	case "M4A ", "M4B ", "M4P ":
		return true
	default:
		return false
	}
}

func probeContainer(ext string, rs io.ReadSeeker) (video, audio, ok bool) {
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return false, false, false
	}
	if ext == ".webm" || ext == ".mkv" || looksEBML(rs) {
		video, audio = probeEBML(rs)
		return video, audio, video || audio
	}
	video, audio = probeISO(rs)
	return video, audio, video || audio
}

func looksEBML(rs io.ReadSeeker) bool {
	var head [4]byte
	n, err := rs.Read(head[:])
	_, _ = rs.Seek(0, io.SeekStart)
	return err == nil && n == 4 && head[0] == 0x1a && head[1] == 0x45 && head[2] == 0xdf && head[3] == 0xa3
}

var (
	videoCodecs = map[string]bool{
		"avc1": true, "avc3": true, "hvc1": true, "hev1": true,
		"vp08": true, "vp09": true, "av01": true, "mp4v": true,
		"encv": true, "dvh1": true, "dvhe": true, "apcn": true,
	}
	audioCodecs = map[string]bool{
		"mp4a": true, "alac": true, "opus": true, "flac": true,
		"samr": true, "sawb": true, "twos": true, "sowt": true,
		".mp3": true, "ac-3": true, "ec-3": true, "lpcm": true,
	}
)

func probeISO(rs io.ReadSeeker) (video, audio bool) {
	end, err := rs.Seek(0, io.SeekEnd)
	if err != nil || end < 8 {
		return false, false
	}
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return false, false
	}
	walkISO(rs, 0, end, 0, false, &video, &audio)
	return video, audio
}

func walkISO(rs io.ReadSeeker, start, end int64, depth int, inSample bool, video, audio *bool) {
	if depth > 8 || *video && *audio {
		return
	}
	pos := start
	for pos+8 <= end {
		if _, err := rs.Seek(pos, io.SeekStart); err != nil {
			return
		}
		var hdr [16]byte
		if _, err := io.ReadFull(rs, hdr[:8]); err != nil {
			return
		}
		size := int64(binary.BigEndian.Uint32(hdr[:4]))
		typ := string(hdr[4:8])
		header := int64(8)
		switch size {
		case 1:
			if _, err := io.ReadFull(rs, hdr[8:16]); err != nil {
				return
			}
			size = int64(binary.BigEndian.Uint64(hdr[8:16]))
			header = 16
		case 0:
			size = end - pos
		}
		if size < header || pos+size > end {
			return
		}
		payload := pos + header
		payloadEnd := pos + size
		switch {
		case typ == "moov" || typ == "trak" || typ == "mdia" || typ == "minf" || typ == "stbl":
			walkISO(rs, payload, payloadEnd, depth+1, false, video, audio)
		case typ == "stsd" && payload+8 <= payloadEnd:
			walkISO(rs, payload+8, payloadEnd, depth+1, true, video, audio)
		case typ == "hdlr":
			noteHandler(rs, payload, payloadEnd, video, audio)
		case inSample && videoCodecs[typ]:
			*video = true
		case inSample && audioCodecs[typ]:
			*audio = true
		}
		pos += size
	}
}

func noteHandler(rs io.ReadSeeker, payload, payloadEnd int64, video, audio *bool) {
	if payload+12 > payloadEnd {
		return
	}
	if _, err := rs.Seek(payload, io.SeekStart); err != nil {
		return
	}
	buf := make([]byte, 12)
	if _, err := io.ReadFull(rs, buf); err != nil {
		return
	}
	for _, off := range []int{4, 8} {
		kind := string(buf[off : off+4])
		if kind == "vide" {
			*video = true
		}
		if kind == "soun" {
			*audio = true
		}
	}
}

func probeEBML(rs io.ReadSeeker) (video, audio bool) {
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return false, false
	}
	videoMarks := [][]byte{
		[]byte("V_VP8"), []byte("V_VP9"), []byte("V_AV1"),
		[]byte("V_MPEG4"), []byte("V_MPEGH"), []byte("V_THEORA"), []byte("V_MJPEG"),
	}
	audioMarks := [][]byte{
		[]byte("A_OPUS"), []byte("A_VORBIS"), []byte("A_AAC"),
		[]byte("A_MPEG"), []byte("A_FLAC"), []byte("A_PCM"), []byte("A_AC3"),
	}
	buf := make([]byte, 256*1024)
	var pending []byte
	var read int64
	const limit = 8 << 20
	for read < limit {
		n, err := rs.Read(buf)
		if n > 0 {
			chunk := append(pending, buf[:n]...)
			if bytesContains(chunk, videoMarks) {
				video = true
			}
			if bytesContains(chunk, audioMarks) {
				audio = true
			}
			if video && audio {
				return video, audio
			}
			if len(chunk) > 32 {
				pending = append([]byte(nil), chunk[len(chunk)-32:]...)
			} else {
				pending = append([]byte(nil), chunk...)
			}
			read += int64(n)
		}
		if err != nil {
			break
		}
	}
	return video, audio
}

func bytesContains(buf []byte, marks [][]byte) bool {
	for _, mark := range marks {
		if indexBytes(buf, mark) >= 0 {
			return true
		}
	}
	return false
}

func indexBytes(buf, mark []byte) int {
	return strings.Index(string(buf), string(mark))
}
