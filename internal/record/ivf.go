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
		case "VP90":
			if w, h, ok := vp9Size(payload); ok {
				return w, h, nil
			}
		case "AV01":
			if w, h, ok := av1Size(payload); ok {
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

// bitReader reads big-endian bits from a byte slice. It latches ok=false on the
// first read past the end and then returns zero for every later read, so a
// truncated header is rejected cleanly instead of panicking.
type bitReader struct {
	b   []byte
	pos int // bit offset
	ok  bool
}

func newBitReader(b []byte) *bitReader { return &bitReader{b: b, ok: true} }

func (r *bitReader) bits(n int) uint64 {
	if !r.ok || n < 0 || n > 64 || r.pos+n > len(r.b)*8 {
		r.ok = false
		return 0
	}
	var v uint64
	for i := 0; i < n; i++ {
		p := r.pos + i
		v = v<<1 | uint64((r.b[p/8]>>uint(7-p%8))&1)
	}
	r.pos += n
	return v
}

// uvlc reads the AV1 unsigned variable-length code used for
// num_ticks_per_picture_minus_1: a run of zero bits, a one bit, then that many
// value bits.
func (r *bitReader) uvlc() uint32 {
	leading := 0
	for r.ok && r.bits(1) == 0 {
		leading++
		if leading >= 32 {
			r.ok = false
			return 0
		}
	}
	if !r.ok {
		return 0
	}
	return uint32(r.bits(leading)) + (uint32(1) << uint(leading)) - 1
}

// leb128 reads the AV1 OBU size field: up to 8 bytes, 7 payload bits each, the
// high bit marks continuation. It returns the value and the bytes consumed.
func leb128(b []byte) (uint64, int, bool) {
	var v uint64
	for i := 0; i < 8; i++ {
		if i >= len(b) {
			return 0, 0, false
		}
		c := b[i]
		v |= uint64(c&0x7f) << (7 * i)
		if c&0x80 == 0 {
			return v, i + 1, true
		}
	}
	// Eight bytes each with the continuation bit set: malformed size field.
	return 0, 0, false
}

// vp9Size reads the size from a VP9 keyframe's uncompressed header (profiles
// 0..3). It returns ok=false for non-keyframes — show_existing_frame or a
// frame_type other than KEY_FRAME — so the caller keeps scanning, and for
// anything truncated or malformed.
func vp9Size(b []byte) (uint16, uint16, bool) {
	r := newBitReader(b)
	if r.bits(2) != 2 { // frame_marker
		return 0, 0, false
	}
	low := r.bits(1)  // profile_low_bit
	high := r.bits(1) // profile_high_bit
	profile := int(low | high<<1)
	if profile == 3 {
		r.bits(1) // reserved_zero
	}
	if r.bits(1) == 1 { // show_existing_frame: no size in this header
		return 0, 0, false
	}
	if r.bits(1) != 0 { // frame_type: 0 = KEY_FRAME
		return 0, 0, false
	}
	r.bits(1)                   // show_frame
	r.bits(1)                   // error_resilient_mode
	if r.bits(24) != 0x498342 { // frame sync code
		return 0, 0, false
	}
	// color_config
	if profile >= 2 {
		r.bits(1) // ten_or_twelve_bit
	}
	colorSpace := r.bits(3)
	if colorSpace != 7 { // not SRGB
		r.bits(1) // color_range
		if profile == 1 || profile == 3 {
			r.bits(1) // subsampling_x
			r.bits(1) // subsampling_y
			r.bits(1) // reserved_zero
		}
	} else if profile == 1 || profile == 3 {
		r.bits(1) // SRGB implies 4:4:4, but a reserved_zero bit is still signalled
	}
	w, h := r.bits(16)+1, r.bits(16)+1 // frame_width_minus_1, frame_height_minus_1
	if !r.ok || w > 65535 || h > 65535 {
		return 0, 0, false
	}
	return uint16(w), uint16(h), true
}

// av1Size walks the OBUs of an AV1 frame payload looking for the sequence
// header (OBU type 1), which carries max_frame_width/height. The size is those
// maxima plus one. ok=false when no sequence header is present or it is
// malformed, so the caller keeps scanning.
//
// This is the declared maximum, not necessarily the coded frame size: a stream
// that uses frame_size_override reports a size at or above the real one. That
// beats the writer's fixed 640x480, and ffprobe still wins whenever it exists.
func av1Size(b []byte) (uint16, uint16, bool) {
	for pos := 0; pos < len(b); {
		hdr := b[pos]
		pos++
		if hdr>>7 != 0 { // obu_forbidden_bit
			return 0, 0, false
		}
		obuType := (hdr >> 3) & 0x0f
		hasExt := (hdr>>2)&1 == 1
		hasSize := (hdr>>1)&1 == 1
		if hasExt {
			if pos >= len(b) {
				return 0, 0, false
			}
			pos++ // obu_extension_header byte
		}
		var payload []byte
		if hasSize {
			size, n, ok := leb128(b[pos:])
			if !ok {
				return 0, 0, false
			}
			pos += n
			if size > uint64(len(b)-pos) {
				return 0, 0, false
			}
			payload = b[pos : pos+int(size)]
			pos += int(size)
		} else {
			// No size field: this OBU runs to the end of the buffer.
			payload = b[pos:]
			pos = len(b)
		}
		if obuType == 1 { // OBU_SEQUENCE_HEADER
			return av1SeqSize(payload)
		}
	}
	return 0, 0, false
}

// av1SeqSize parses an AV1 sequence header OBU payload down to the frame size
// fields, skipping timing/decoder-model/operating-point syntax on the way.
func av1SeqSize(b []byte) (uint16, uint16, bool) {
	r := newBitReader(b)
	if r.bits(3) > 2 { // seq_profile
		return 0, 0, false
	}
	r.bits(1)           // still_picture
	if r.bits(1) == 1 { // reduced_still_picture_header
		r.bits(5) // seq_level_idx[0]
	} else {
		decoderModel := uint64(0)
		bufDelayLen := 0
		if r.bits(1) == 1 { // timing_info_present_flag
			r.bits(32)          // num_units_in_tick
			r.bits(32)          // time_scale
			if r.bits(1) == 1 { // equal_picture_interval
				r.uvlc() // num_ticks_per_picture_minus_1
			}
			decoderModel = r.bits(1) // decoder_model_info_present_flag
			if decoderModel == 1 {
				bufDelayLen = int(r.bits(5)) + 1 // buffer_delay_length_minus_1
				r.bits(32)                       // num_units_in_decoding_tick
				r.bits(5)                        // buffer_removal_time_length_minus_1
				r.bits(5)                        // frame_presentation_time_length_minus_1
			}
		}
		initialDisplay := r.bits(1) // initial_display_delay_present_flag
		ops := r.bits(5)            // operating_points_cnt_minus_1
		for i := uint64(0); i <= ops && r.ok; i++ {
			r.bits(12)         // operating_point_idc
			if r.bits(5) > 7 { // seq_level_idx; seq_tier only for levels above 7
				r.bits(1) // seq_tier
			}
			if decoderModel == 1 && r.bits(1) == 1 { // decoder_model_present_for_this_op
				r.bits(bufDelayLen) // decoder_buffer_delay
				r.bits(bufDelayLen) // encoder_buffer_delay
				r.bits(1)           // low_delay_mode_flag
			}
			if initialDisplay == 1 && r.bits(1) == 1 { // initial_display_delay_present_for_this_op
				r.bits(4) // initial_display_delay_minus_1
			}
		}
	}
	wBits := int(r.bits(4)) + 1              // frame_width_bits_minus_1
	hBits := int(r.bits(4)) + 1              // frame_height_bits_minus_1
	w, h := r.bits(wBits)+1, r.bits(hBits)+1 // max_frame_width/height_minus_1
	if !r.ok || w > 65535 || h > 65535 {
		return 0, 0, false
	}
	return uint16(w), uint16(h), true
}
