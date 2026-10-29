package media

import (
	"bytes"
	"encoding/binary"
	"testing"

	"hearth/internal/store"
)

func TestClassifyAudioOnlyMP4(t *testing.T) {
	body := join(isoBox("ftyp", []byte("isom\x00\x00\x00\x00isom")), isoBox("moov", isoBox("trak", isoBox("mdia", handler("soun")))))
	mimeType, kind := Classify("voice.mp4", body[:min(512, len(body))], bytes.NewReader(body))
	if kind != store.KindAudio || mimeType != "audio/mp4" {
		t.Fatalf("got %s %s", mimeType, kind)
	}
}

func TestClassifyM4ABrand(t *testing.T) {
	body := isoBox("ftyp", []byte("M4A \x00\x00\x00\x00M4A "))
	mimeType, kind := Classify("clip.m4a", body, bytes.NewReader(body))
	if kind != store.KindAudio || mimeType != "audio/mp4" {
		t.Fatalf("got %s %s", mimeType, kind)
	}
}

func TestClassifyVideoMP4(t *testing.T) {
	body := join(
		isoBox("ftyp", []byte("isom\x00\x00\x00\x00isom")),
		isoBox("moov", isoBox("trak", isoBox("mdia", join(handler("vide"), isoBox("minf", isoBox("stbl", stsd("avc1"))))))),
	)
	mimeType, kind := Classify("movie.mp4", body[:min(512, len(body))], bytes.NewReader(body))
	if kind != store.KindVideo || mimeType != "video/mp4" {
		t.Fatalf("got %s %s", mimeType, kind)
	}
}

func TestClassifyWebMAudio(t *testing.T) {
	body := append([]byte{0x1a, 0x45, 0xdf, 0xa3}, []byte("A_OPUS")...)
	mimeType, kind := Classify("note.webm", body, bytes.NewReader(body))
	if kind != store.KindAudio || mimeType != "audio/webm" {
		t.Fatalf("got %s %s", mimeType, kind)
	}
}

func TestClassifySniffedAudioAndImage(t *testing.T) {
	_, kind := Classify("a.bin", []byte("ID3\x03\x00"), bytes.NewReader([]byte("ID3\x03\x00")))
	if kind != store.KindAudio {
		t.Fatalf("mp3 kind %s", kind)
	}
	_, kind = Classify("a.jpg", []byte{0xff, 0xd8, 0xff}, nil)
	if kind != store.KindImage {
		t.Fatalf("jpeg kind %s", kind)
	}
	_, kind = Classify("a.pdf", []byte("%PDF-1.4"), nil)
	if kind != store.KindOther {
		t.Fatalf("pdf kind %s", kind)
	}
}

func join(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

func handler(kind string) []byte {
	payload := make([]byte, 12)
	copy(payload[8:], kind)
	return isoBox("hdlr", payload)
}

func stsd(codec string) []byte {
	payload := make([]byte, 8)
	return isoBox("stsd", append(payload, isoBox(codec, []byte{0})...))
}

func isoBox(typ string, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(out[:4], uint32(len(out)))
	copy(out[4:8], typ)
	copy(out[8:], payload)
	return out
}
