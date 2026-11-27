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

// bitWriter packs bits MSB-first, the mirror of the bitReader in ivf.go, so the
// tests can assemble synthetic VP9/AV1 headers bit by bit.
type bitWriter struct {
	b   []byte
	cur byte
	n   int // bits already packed into cur
}

func (w *bitWriter) bits(v uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		w.cur = w.cur<<1 | byte((v>>uint(i))&1)
		w.n++
		if w.n == 8 {
			w.b = append(w.b, w.cur)
			w.cur = 0
			w.n = 0
		}
	}
}

// uvlc writes the AV1 unsigned variable-length code for v.
func (w *bitWriter) uvlc(v uint32) {
	leading := 0
	for leading < 31 && v >= (uint32(1)<<uint(leading+1))-1 {
		leading++
	}
	for i := 0; i < leading; i++ {
		w.bits(0, 1)
	}
	w.bits(1, 1) // done bit
	w.bits(uint64(v-((uint32(1)<<uint(leading))-1)), leading)
}

func (w *bitWriter) bytes() []byte {
	out := append([]byte(nil), w.b...)
	if w.n > 0 {
		out = append(out, w.cur<<uint(8-w.n))
	}
	return out
}

// obu wraps payload in an OBU header. withSize adds a leb128 size field;
// otherwise the payload runs to the end of the buffer (low-overhead format).
func obu(obuType byte, ext, withSize bool, payload []byte) []byte {
	hdr := (obuType & 0x0f) << 3
	if ext {
		hdr |= 1 << 2
	}
	if withSize {
		hdr |= 1 << 1
	}
	out := []byte{hdr}
	if ext {
		out = append(out, 0x00) // extension header: temporal_id=0, spatial_id=0
	}
	if withSize {
		v := len(payload)
		for {
			c := byte(v & 0x7f)
			v >>= 7
			if v != 0 {
				c |= 0x80
			}
			out = append(out, c)
			if v == 0 {
				break
			}
		}
	}
	return append(out, payload...)
}

// bitsNeeded is the fewest bits that represent v (at least 1).
func bitsNeeded(v uint16) int {
	n := 1
	for n < 16 && uint16(1)<<uint(n) <= v {
		n++
	}
	return n
}

// vp9Keyframe builds a VP9 keyframe uncompressed header for profile 0..3 and
// the given color space, declaring w x h.
func vp9Keyframe(profile int, colorSpace uint64, w, h uint16) []byte {
	var bw bitWriter
	bw.bits(2, 2)                    // frame_marker
	bw.bits(uint64(profile&1), 1)    // profile_low_bit
	bw.bits(uint64(profile>>1&1), 1) // profile_high_bit
	if profile == 3 {
		bw.bits(0, 1) // reserved_zero
	}
	bw.bits(0, 1)         // show_existing_frame = 0
	bw.bits(0, 1)         // frame_type = KEY_FRAME
	bw.bits(1, 1)         // show_frame = 1
	bw.bits(0, 1)         // error_resilient_mode = 0
	bw.bits(0x498342, 24) // sync code
	if profile >= 2 {
		bw.bits(0, 1) // ten_or_twelve_bit = 10-bit
	}
	bw.bits(colorSpace, 3)
	if colorSpace != 7 {
		bw.bits(0, 1) // color_range
		if profile == 1 || profile == 3 {
			bw.bits(0, 1) // subsampling_x
			bw.bits(0, 1) // subsampling_y
			bw.bits(0, 1) // reserved_zero
		}
	} else if profile == 1 || profile == 3 {
		bw.bits(0, 1) // reserved_zero (SRGB implies 4:4:4)
	}
	bw.bits(uint64(w-1), 16)
	bw.bits(uint64(h-1), 16)
	return bw.bytes()
}

func vp9InterFrame() []byte {
	var bw bitWriter
	bw.bits(2, 2) // frame_marker
	bw.bits(0, 1) // profile_low (profile 0)
	bw.bits(0, 1) // profile_high
	bw.bits(0, 1) // show_existing_frame = 0
	bw.bits(1, 1) // frame_type = INTER_FRAME, not a keyframe
	bw.bits(1, 1) // show_frame
	bw.bits(0, 1) // error_resilient_mode
	return bw.bytes()
}

func vp9ShowExistingFrame() []byte {
	var bw bitWriter
	bw.bits(2, 2) // frame_marker
	bw.bits(0, 1) // profile_low
	bw.bits(0, 1) // profile_high
	bw.bits(1, 1) // show_existing_frame = 1
	return bw.bytes()
}

// av1WriteSize writes frame_width/height_bits_minus_1 and the maxima.
func av1WriteSize(w *bitWriter, width, height uint16) {
	wb, hb := bitsNeeded(width-1), bitsNeeded(height-1)
	w.bits(uint64(wb-1), 4) // frame_width_bits_minus_1
	w.bits(uint64(hb-1), 4) // frame_height_bits_minus_1
	w.bits(uint64(width-1), wb)
	w.bits(uint64(height-1), hb)
}

// av1SeqReduced builds a reduced_still_picture_header sequence header.
func av1SeqReduced(w, h uint16) []byte {
	var bw bitWriter
	bw.bits(0, 3) // seq_profile = 0
	bw.bits(1, 1) // still_picture = 1
	bw.bits(1, 1) // reduced_still_picture_header = 1
	bw.bits(0, 5) // seq_level_idx[0]
	av1WriteSize(&bw, w, h)
	bw.bits(1, 1) // trailing_one_bit
	return bw.bytes()
}

// av1SeqFull builds the full non-reduced path: timing info, decoder model and
// two operating points with tier and operating_parameters_info.
func av1SeqFull(w, h uint16) []byte {
	var bw bitWriter
	bw.bits(0, 3)      // seq_profile
	bw.bits(0, 1)      // still_picture = 0
	bw.bits(0, 1)      // reduced_still_picture_header = 0
	bw.bits(1, 1)      // timing_info_present = 1
	bw.bits(1, 32)     // num_units_in_tick
	bw.bits(30000, 32) // time_scale
	bw.bits(1, 1)      // equal_picture_interval = 1
	bw.uvlc(0)         // num_ticks_per_picture_minus_1
	bw.bits(1, 1)      // decoder_model_info_present = 1
	bw.bits(9, 5)      // buffer_delay_length_minus_1 -> n = 10
	bw.bits(1, 32)     // num_units_in_decoding_tick
	bw.bits(9, 5)      // buffer_removal_time_length_minus_1
	bw.bits(9, 5)      // frame_presentation_time_length_minus_1
	bw.bits(1, 1)      // initial_display_delay_present = 1
	bw.bits(1, 5)      // operating_points_cnt_minus_1 = 1 -> 2 ops
	for op := 0; op < 2; op++ {
		bw.bits(0, 12)   // operating_point_idc
		bw.bits(8, 5)    // seq_level_idx = 8 (> 7 -> tier present)
		bw.bits(0, 1)    // seq_tier
		bw.bits(1, 1)    // decoder_model_present_for_this_op = 1
		bw.bits(100, 10) // decoder_buffer_delay
		bw.bits(100, 10) // encoder_buffer_delay
		bw.bits(0, 1)    // low_delay_mode_flag
		bw.bits(1, 1)    // initial_display_delay_present_for_this_op = 1
		bw.bits(7, 4)    // initial_display_delay_minus_1
	}
	av1WriteSize(&bw, w, h)
	bw.bits(1, 1) // trailing_one_bit
	return bw.bytes()
}

// av1SeqNoTiming builds the simplest non-reduced header: no timing info, one
// operating point at the given level. seq_tier is present only when level > 7.
func av1SeqNoTiming(w, h uint16, level uint64) []byte {
	var bw bitWriter
	bw.bits(0, 3)     // seq_profile
	bw.bits(0, 1)     // still_picture
	bw.bits(0, 1)     // reduced_still_picture_header = 0
	bw.bits(0, 1)     // timing_info_present = 0
	bw.bits(0, 1)     // initial_display_delay_present = 0
	bw.bits(0, 5)     // operating_points_cnt_minus_1 = 0 -> 1 op
	bw.bits(0, 12)    // operating_point_idc
	bw.bits(level, 5) // seq_level_idx
	if level > 7 {
		bw.bits(0, 1) // seq_tier
	}
	av1WriteSize(&bw, w, h)
	bw.bits(1, 1) // trailing_one_bit
	return bw.bytes()
}

func TestVP9Size(t *testing.T) {
	cases := []struct {
		profile int
		cs      uint64
		w, h    uint16
	}{
		{0, 2, 1280, 720},
		{0, 2, 1920, 1080},
		{0, 2, 640, 480},
		{0, 2, 1080, 1920}, // portrait
		{0, 2, 1281, 721},  // odd
		{0, 2, 1, 1},
		{2, 2, 1920, 1080},
		{2, 2, 3840, 2160},
		{2, 2, 1081, 1921}, // odd portrait
		{1, 2, 1280, 720},
		{3, 2, 1280, 720},
		{1, 7, 640, 480},   // SRGB
		{3, 7, 1920, 1080}, // SRGB
	}
	for _, c := range cases {
		b := vp9Keyframe(c.profile, c.cs, c.w, c.h)
		if w, h, ok := vp9Size(b); !ok || w != c.w || h != c.h {
			t.Errorf("vp9Size(profile=%d, cs=%d) = %dx%d ok=%v, want %dx%d",
				c.profile, c.cs, w, h, ok, c.w, c.h)
		}
	}
}

func TestVP9SizeNonKeyframe(t *testing.T) {
	if _, _, ok := vp9Size(vp9InterFrame()); ok {
		t.Error("vp9Size must reject an inter frame")
	}
	if _, _, ok := vp9Size(vp9ShowExistingFrame()); ok {
		t.Error("vp9Size must reject show_existing_frame")
	}
}

func TestAV1Size(t *testing.T) {
	cases := []struct {
		name string
		seq  []byte
		w, h uint16
	}{
		{"reduced", av1SeqReduced(1280, 720), 1280, 720},
		{"reduced_odd_portrait", av1SeqReduced(1081, 1921), 1081, 1921},
		{"reduced_4k", av1SeqReduced(3840, 2160), 3840, 2160},
		{"full", av1SeqFull(1920, 1080), 1920, 1080},
		{"full_portrait", av1SeqFull(1080, 1920), 1080, 1920},
		{"no_timing_level4", av1SeqNoTiming(640, 480, 4), 640, 480},
		{"no_timing_level8_tier", av1SeqNoTiming(1280, 720, 8), 1280, 720},
	}
	for _, c := range cases {
		for _, mode := range []struct {
			label      string
			ext, sized bool
		}{
			{"size_field", false, true},
			{"extension", true, true},
			{"no_size_field", false, false},
		} {
			b := obu(1, mode.ext, mode.sized, c.seq)
			if w, h, ok := av1Size(b); !ok || w != c.w || h != c.h {
				t.Errorf("%s (%s): av1Size = %dx%d ok=%v, want %dx%d",
					c.name, mode.label, w, h, ok, c.w, c.h)
			}
		}
	}
}

func TestAV1SizeScansOBUs(t *testing.T) {
	seq := obu(1, false, true, av1SeqReduced(1280, 720))
	td := obu(2, false, true, []byte{}) // temporal delimiter, empty payload
	payload := append(append([]byte(nil), td...), seq...)
	if w, h, ok := av1Size(payload); !ok || w != 1280 || h != 720 {
		t.Errorf("av1Size(TD+seq) = %dx%d ok=%v, want 1280x720", w, h, ok)
	}
	if _, _, ok := av1Size(td); ok {
		t.Error("av1Size must return ok=false when no sequence header is present")
	}
}

func TestFixIVFSizeVP9(t *testing.T) {
	path := ivf(t, "VP90", 640, 480, vp9Keyframe(0, 2, 1920, 1080))
	if err := fixIVFSize(path, ""); err != nil {
		t.Fatalf("fixIVFSize: %v", err)
	}
	if w, h := declaredSize(t, path); w != 1920 || h != 1080 {
		t.Fatalf("header declares %dx%d, want 1920x1080", w, h)
	}
}

func TestFixIVFSizeVP9Profile2(t *testing.T) {
	path := ivf(t, "VP90", 640, 480, vp9Keyframe(2, 2, 1081, 1921))
	if err := fixIVFSize(path, ""); err != nil {
		t.Fatalf("fixIVFSize: %v", err)
	}
	if w, h := declaredSize(t, path); w != 1081 || h != 1921 {
		t.Fatalf("header declares %dx%d, want 1081x1921", w, h)
	}
}

func TestFixIVFSizeVP9SkipsNonKeyframe(t *testing.T) {
	path := ivf(t, "VP90", 640, 480,
		vp9InterFrame(), vp9ShowExistingFrame(), vp9Keyframe(0, 2, 1280, 720))
	if err := fixIVFSize(path, ""); err != nil {
		t.Fatalf("fixIVFSize: %v", err)
	}
	if w, h := declaredSize(t, path); w != 1280 || h != 720 {
		t.Fatalf("header declares %dx%d, want 1280x720", w, h)
	}
}

func TestFixIVFSizeVP9NoKeyframeEOF(t *testing.T) {
	path := ivf(t, "VP90", 640, 480, vp9InterFrame(), vp9InterFrame())
	if err := fixIVFSize(path, ""); err == nil {
		t.Fatal("expected an EOF error when no keyframe exists")
	}
	if w, h := declaredSize(t, path); w != 640 || h != 480 {
		t.Fatalf("header changed to %dx%d without a keyframe", w, h)
	}
}

func TestFixIVFSizeAV1(t *testing.T) {
	seq := obu(1, false, true, av1SeqReduced(1920, 1080))
	path := ivf(t, "AV01", 640, 480, seq)
	if err := fixIVFSize(path, ""); err != nil {
		t.Fatalf("fixIVFSize: %v", err)
	}
	if w, h := declaredSize(t, path); w != 1920 || h != 1080 {
		t.Fatalf("header declares %dx%d, want 1920x1080", w, h)
	}
}

func TestFixIVFSizeAV1Full(t *testing.T) {
	seq := obu(1, true, true, av1SeqFull(1280, 720))
	path := ivf(t, "AV01", 640, 480, seq)
	if err := fixIVFSize(path, ""); err != nil {
		t.Fatalf("fixIVFSize: %v", err)
	}
	if w, h := declaredSize(t, path); w != 1280 || h != 720 {
		t.Fatalf("header declares %dx%d, want 1280x720", w, h)
	}
}

func TestFixIVFSizeAV1SkipsFrameWithoutSeqHeader(t *testing.T) {
	noSeq := obu(2, false, true, []byte{}) // TD only
	seq := obu(1, false, true, av1SeqReduced(1080, 1920))
	path := ivf(t, "AV01", 640, 480, noSeq, seq)
	if err := fixIVFSize(path, ""); err != nil {
		t.Fatalf("fixIVFSize: %v", err)
	}
	if w, h := declaredSize(t, path); w != 1080 || h != 1920 {
		t.Fatalf("header declares %dx%d, want 1080x1920", w, h)
	}
}

func TestFixIVFSizeAV1NoSeqHeaderEOF(t *testing.T) {
	path := ivf(t, "AV01", 640, 480,
		obu(2, false, true, []byte{}), obu(2, false, true, []byte{}))
	if err := fixIVFSize(path, ""); err == nil {
		t.Fatal("expected an EOF error when no sequence header exists")
	}
	if w, h := declaredSize(t, path); w != 640 || h != 480 {
		t.Fatalf("header changed to %dx%d without a sequence header", w, h)
	}
}

func TestFixIVFSizeUnsupportedFourcc(t *testing.T) {
	path := ivf(t, "H264", 640, 480, vp8Keyframe(1280, 720))
	err := fixIVFSize(path, "")
	if err == nil || err.Error() != "unsupported ivf fourcc H264" {
		t.Fatalf("fixIVFSize = %v, want unsupported fourcc error", err)
	}
	if w, h := declaredSize(t, path); w != 640 || h != 480 {
		t.Fatalf("header changed to %dx%d for an unsupported fourcc", w, h)
	}
}

func TestFixIVFSizeTruncatedFileNoPanic(t *testing.T) {
	path := ivf(t, "VP90", 640, 480, vp9Keyframe(0, 2, 1280, 720))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data[:len(data)-5], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fixIVFSize(path, ""); err == nil {
		t.Fatal("expected an error for a truncated file")
	}
	if w, h := declaredSize(t, path); w != 640 || h != 480 {
		t.Fatalf("header changed to %dx%d on a truncated file", w, h)
	}
}

func TestSizeParsersTruncatedNoPanic(t *testing.T) {
	vp9 := vp9Keyframe(0, 2, 1280, 720)
	av1 := obu(1, false, true, av1SeqFull(1920, 1080))
	// Every proper prefix must be rejected without panicking; the size fields
	// sit at the end of the header, so no truncated prefix can yield a size.
	for i := 0; i < len(vp9); i++ {
		if _, _, ok := vp9Size(vp9[:i]); ok {
			t.Errorf("vp9Size accepted a truncated prefix of %d bytes", i)
		}
	}
	for i := 0; i < len(av1); i++ {
		if _, _, ok := av1Size(av1[:i]); ok {
			t.Errorf("av1Size accepted a truncated prefix of %d bytes", i)
		}
	}
	for _, empty := range [][]byte{nil, {}} {
		if _, _, ok := vp9Size(empty); ok {
			t.Error("vp9Size(empty) must not succeed")
		}
		if _, _, ok := av1Size(empty); ok {
			t.Error("av1Size(empty) must not succeed")
		}
	}
}

func TestLEB128RejectsOverlongEncoding(t *testing.T) {
	// Nine bytes where the first eight all carry the continuation bit: the AV1
	// OBU size field is malformed and must be rejected, not silently accepted.
	bad := []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x01}
	if _, _, ok := leb128(bad); ok {
		t.Fatal("leb128 accepted eight continuation bytes")
	}
	if v, n, ok := leb128([]byte{0x85, 0x01}); !ok || v != 133 || n != 2 {
		t.Fatalf("leb128(0x85,0x01) = %d, %d, %v; want 133, 2, true", v, n, ok)
	}
	if _, _, ok := leb128(nil); ok {
		t.Fatal("leb128(nil) must not succeed")
	}
}
