package record

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// IVF is the container pion writes for VP8/VP9/AV1: a 32 byte DKIF header that
// declares the frame size, then one 12 byte header per frame.
const ivfHeaderSize = 32

// fixIVFSize patches the frame size declared in an IVF header with the size the
// bitstream actually carries.
//
// pion's ivfwriter never looks at the bitstream: it always declares 640x480
// unless the caller passes WithWidthAndHeight, which needs the size before the
// first frame arrives. The muxer later runs with -c copy, so that declaration
// ended up in the .mkv and every recording played back as 640x480, even when
// the guest published 1920x1080.
//
// The real size is what a decoder sees, so ask ffprobe for one decoded frame.
// Without ffprobe (no muxing either) fall back to reading the first keyframe.
func fixIVFSize(path, ffprobe string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	header := make([]byte, ivfHeaderSize)
	if _, err := io.ReadFull(f, header); err != nil {
		return err
	}
	if string(header[0:4]) != "DKIF" {
		return errors.New("not an ivf file")
	}
	fourcc := string(header[8:12])

	width, height, err := decodedSize(path, ffprobe)
	if err != nil || width == 0 || height == 0 {
		if _, err := f.Seek(ivfHeaderSize, io.SeekStart); err != nil {
			return err
		}
		width, height, err = keyframeSize(f, fourcc)
		if err != nil {
			return err
		}
	}
	if width == 0 || height == 0 {
		return errors.New("could not determine the frame size")
	}
	binary.LittleEndian.PutUint16(header[12:14], width)
	binary.LittleEndian.PutUint16(header[14:16], height)
	if _, err := f.WriteAt(header, 0); err != nil {
		return err
	}
	return nil
}

// decodedSize asks ffprobe for the size of the first decoded frame: the probe
// reads the bitstream, not the container, so it is correct even when the header
// lies.
func decodedSize(path, ffprobe string) (uint16, uint16, error) {
	if ffprobe == "" {
		return 0, 0, errors.New("no ffprobe")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0",
		"-read_intervals", "%+#1", "-show_entries", "frame=width,height",
		"-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0, 0, err
	}
	return parseProbeSize(string(out))
}

// parseProbeSize reads the two numbers ffprobe prints, one per line.
func parseProbeSize(out string) (uint16, uint16, error) {
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return 0, 0, errors.New("ffprobe returned no frame size")
	}
	w, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, err
	}
	h, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, err
	}
	if w <= 0 || h <= 0 || w > 8192 || h > 8192 {
		return 0, 0, errors.New("ffprobe returned an implausible frame size")
	}
	return uint16(w), uint16(h), nil
}

// keyframeSize walks the frames until the first keyframe and decodes its size.
// Files without a keyframe keep whatever the writer declared.
func keyframeSize(f *os.File, fourcc string) (uint16, uint16, error) {
	frame := make([]byte, 12)
	var payload []byte
	for {
		if _, err := io.ReadFull(f, frame); err != nil {
			return 0, 0, err
		}
		size := binary.LittleEndian.Uint32(frame[0:4])
		if size == 0 || size > 32<<20 {
			return 0, 0, errors.New("unexpected ivf frame size")
		}
		if cap(payload) < int(size) {
			payload = make([]byte, size)
		}
		payload = payload[:size]
		if _, err := io.ReadFull(f, payload); err != nil {
			return 0, 0, err
		}
		switch fourcc {
		case "VP80":
			if w, h, ok := vp8Size(payload); ok {
				return w, h, nil
			}
		default:
			return 0, 0, errors.New("unsupported ivf fourcc " + fourcc)
		}
	}
}

// vp8Size reads the size from a VP8 keyframe: frame tag, start code 9d 01 2a,
// then 14 bit width and height.
func vp8Size(b []byte) (uint16, uint16, bool) {
	if len(b) < 10 || b[0]&1 != 0 { // bit 0 of the frame tag: 0 = keyframe
		return 0, 0, false
	}
	if b[3] != 0x9d || b[4] != 0x01 || b[5] != 0x2a {
		return 0, 0, false
	}
	w := binary.LittleEndian.Uint16(b[6:8]) & 0x3fff
	h := binary.LittleEndian.Uint16(b[8:10]) & 0x3fff
	return w, h, true
}
