package record

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// ivf builds a DKIF file: the writer declares declaredW x declaredH, the frames
// carry the given payloads.
func ivf(t *testing.T, fourcc string, declaredW, declaredH uint16, frames ...[]byte) string {
	t.Helper()
	header := make([]byte, 32)
	copy(header[0:4], "DKIF")
	binary.LittleEndian.PutUint16(header[4:6], 0)
	binary.LittleEndian.PutUint16(header[6:8], 32)
	copy(header[8:12], fourcc)
	binary.LittleEndian.PutUint16(header[12:14], declaredW)
	binary.LittleEndian.PutUint16(header[14:16], declaredH)
	body := header
	for _, frame := range frames {
		head := make([]byte, 12)
		binary.LittleEndian.PutUint32(head[0:4], uint32(len(frame)))
		body = append(body, head...)
		body = append(body, frame...)
	}
	path := filepath.Join(t.TempDir(), "clip.ivf")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func vp8Keyframe(w, h uint16) []byte {
	frame := make([]byte, 16)
	frame[3], frame[4], frame[5] = 0x9d, 0x01, 0x2a
	binary.LittleEndian.PutUint16(frame[6:8], w)
	binary.LittleEndian.PutUint16(frame[8:10], h)
	return frame
}

func declaredSize(t *testing.T, path string) (uint16, uint16) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return binary.LittleEndian.Uint16(data[12:14]), binary.LittleEndian.Uint16(data[14:16])
}

func TestFixIVFSizeUsesTheKeyframeSize(t *testing.T) {
	path := ivf(t, "VP80", 640, 480, vp8Keyframe(1280, 720))
	if err := fixIVFSize(path, ""); err != nil {
		t.Fatalf("fixIVFSize: %v", err)
	}
	if w, h := declaredSize(t, path); w != 1280 || h != 720 {
		t.Fatalf("header declares %dx%d, want 1280x720", w, h)
	}
}

func TestParseProbeSize(t *testing.T) {
	w, h, err := parseProbeSize("1920\n1080\n")
	if err != nil || w != 1920 || h != 1080 {
		t.Fatalf("parseProbeSize = %d x %d, %v", w, h, err)
	}
	if _, _, err := parseProbeSize(""); err == nil {
		t.Fatal("empty ffprobe output must be an error")
	}
	if _, _, err := parseProbeSize("0\n0\n"); err == nil {
		t.Fatal("a zero frame size must be rejected")
	}
}

func TestFixIVFSizeSkipsFilesWithoutKeyframe(t *testing.T) {
	delta := []byte{0x11, 0x00, 0x00, 0x00, 0x00, 0x00}
	path := ivf(t, "VP80", 640, 480, delta)
	if err := fixIVFSize(path, ""); err != nil && err.Error() != "EOF" {
		t.Fatalf("fixIVFSize: %v", err)
	}
	if w, h := declaredSize(t, path); w != 640 || h != 480 {
		t.Fatalf("header changed to %dx%d without a keyframe", w, h)
	}
}
