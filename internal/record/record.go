// Package record writes forwarded RTP to disk without re-encoding
// (ISO-BMFF muxing is delegated to ffmpeg when it is available).
//
// Each guest is recorded as a separate, full quality ISO track - the
// "local multi-track recording" feature of tools like Riverside/StreamYard,
// done server side so a weak guest uplink never ruins the program.
package record

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/h264writer"
	"github.com/pion/webrtc/v4/pkg/media/ivfwriter"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"

	"github.com/webr0m/streamwebrom/internal/sfu"
)

type rtpWriter interface {
	WriteRTP(*rtp.Packet) error
	Close() error
}

// lockedWriter makes media writers safe: the forwarder may call from its own goroutine
// while Stop closes the file.
type lockedWriter struct {
	mu     sync.Mutex
	w      rtpWriter
	closed bool
}

func (l *lockedWriter) WriteRTP(p *rtp.Packet) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return io.ErrClosedPipe
	}
	return l.w.WriteRTP(p)
}

func (l *lockedWriter) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	return l.w.Close()
}

// Recording is an in-progress capture of one stream.
//
// Tracks can appear after Start - a guest turning on the screen share or a
// second camera mid-show - so the track list is live: watchTracks docks the
// late ones and Stop has to wait for it before closing the files.
type Recording struct {
	Stream  string    `json:"stream"`
	Files   []string  `json:"files"`
	Started time.Time `json:"started"`
	Output  string    `json:"output,omitempty"`

	mu      sync.Mutex
	writers []*lockedWriter
	removes []func()
	taken   map[string]bool     // forwarder IDs already being recorded
	names   map[string]int      // tracks per kind: two cameras need two files
	seen    *sfu.Stream         // source of TracksChanged notifications
	stop    chan struct{}       // closes the track watcher
	done    chan struct{}       // the watcher has stopped docking tracks
	log     *slog.Logger
}

// Recorder manages recordings under a directory.
type Recorder struct {
	Dir    string
	FFmpeg string
	Log    *slog.Logger

	mu     sync.Mutex
	active map[string]*Recording // key room/stream
}

// New creates a recorder.
func New(dir, ffmpeg string) *Recorder {
	return &Recorder{Dir: dir, FFmpeg: ffmpeg, active: map[string]*Recording{}}
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// recordedBase matches the kind suffix trackName appends: -video, -audio2...
var recordedBase = regexp.MustCompile(`-(video|audio)[0-9]*$`)

// SafeName strips anything that could escape the recordings directory.
func SafeName(s string) string {
	s = unsafeChars.ReplaceAllString(s, "_")
	if len(s) > 64 {
		s = s[:64]
	}
	if s == "" {
		s = "x"
	}
	return s
}

// Start begins recording every track of s and keeps docking the tracks that
// appear later.
func (r *Recorder) Start(room string, s *sfu.Stream) (*Recording, error) {
	key := room + "/" + s.ID
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.active[key]; ok {
		return nil, errors.New("already recording")
	}
	dir := filepath.Join(r.Dir, "recordings", SafeName(room))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ts := time.Now().Format("20060102-150405.000000000")
	rec := &Recording{Stream: s.ID, Started: time.Now(), taken: map[string]bool{}, names: map[string]int{},
		seen: s, stop: make(chan struct{}), done: make(chan struct{}), log: r.log()}
	for _, f := range s.Forwarders() {
		if err := rec.attach(dir, ts, f); err != nil {
			rec.stopAll()
			return nil, err
		}
	}
	if len(rec.writers) == 0 {
		return nil, errors.New("stream has no tracks yet")
	}
	r.active[key] = rec
	// The publisher can add a track between the snapshot above and this
	// signal; the watcher also re-reads the list when it starts, and a late
	// forwarder stays in Forwarders, so nothing is lost either way.
	s.OnClose(func() { _, _ = r.Stop(room, s.ID) })
	go rec.watchTracks()
	return rec, nil
}

func (r *Recorder) log() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}

// attach opens the file of f and docks the writer onto the forwarder.
// The caller holds rec.mu.
func (rec *Recording) attach(dir, ts string, f *sfu.Forwarder) error {
	rec.taken[f.ID] = true
	base := filepath.Join(dir, rec.trackName(ts, f))
	w, path, err := open(base, f.Codec)
	if err != nil {
		return err
	}
	lw := &lockedWriter{w: w}
	rec.writers = append(rec.writers, lw)
	rec.Files = append(rec.Files, path)
	rec.removes = append(rec.removes, f.AddSink(lw))
	return nil
}

// trackName names one track's file. The first track of a kind keeps the plain
// <stream>-<ts>-<kind> name every existing path and glob expects; a second
// camera or screen share gets -video2, -video3 and so on instead of writing
// two codecs into one file.
func (rec *Recording) trackName(ts string, f *sfu.Forwarder) string {
	kind := f.Kind.String()
	rec.names[kind]++
	n := rec.names[kind]
	if n == 1 {
		return fmt.Sprintf("%s-%s-%s", SafeName(rec.Stream), ts, kind)
	}
	return fmt.Sprintf("%s-%s-%s%d", SafeName(rec.Stream), ts, kind, n)
}

// watchTracks docks the tracks that come up after Start: a screen share or a
// second camera switched on in the middle of a show.
func (rec *Recording) watchTracks() {
	defer close(rec.done)
	for {
		rec.dock()
		select {
		case <-rec.stop:
			return
		case <-rec.seen.Done():
			return
		case <-rec.seen.TracksChanged():
		}
	}
}

// dock adds the forwarders that are not recorded yet. A track that cannot be
// opened is a warning: one exotic codec must not lose the other tracks.
func (rec *Recording) dock() {
	ts := rec.Started.Format("20060102-150405.000000000")
	for _, f := range rec.seen.Forwarders() {
		rec.mu.Lock()
		if rec.taken[f.ID] {
			rec.mu.Unlock()
			continue
		}
		err := rec.attach(filepath.Dir(rec.Files[0]), ts, f)
		rec.mu.Unlock()
		if err != nil {
			rec.log.Warn("record: late track skipped", "stream", rec.Stream, "track", f.ID, "codec", f.Codec.MimeType, "err", err)
			continue
		}
		rec.log.Info("record: late track", "stream", rec.Stream, "track", f.ID, "kind", f.Kind.String(), "codec", f.Codec.MimeType)
	}
}

func open(base string, c webrtc.RTPCodecCapability) (rtpWriter, string, error) {
	switch strings.ToLower(c.MimeType) {
	case strings.ToLower(webrtc.MimeTypeOpus):
		p := base + ".ogg"
		w, err := oggwriter.New(p, c.ClockRate, max16(c.Channels, 2))
		return w, p, err
	case strings.ToLower(webrtc.MimeTypeVP8):
		p := base + ".ivf"
		w, err := ivfwriter.New(p, ivfwriter.WithCodec(webrtc.MimeTypeVP8))
		return w, p, err
	case strings.ToLower(webrtc.MimeTypeVP9):
		p := base + ".ivf"
		w, err := ivfwriter.New(p, ivfwriter.WithCodec(webrtc.MimeTypeVP9))
		return w, p, err
	case strings.ToLower(webrtc.MimeTypeAV1):
		p := base + ".ivf"
		w, err := ivfwriter.New(p, ivfwriter.WithCodec(webrtc.MimeTypeAV1))
		return w, p, err
	case strings.ToLower(webrtc.MimeTypeH264):
		p := base + ".h264"
		w, err := h264writer.New(p)
		return w, p, err
	}
	return nil, "", fmt.Errorf("cannot record codec %s", c.MimeType)
}

func max16(v uint16, d uint16) uint16 {
	if v == 0 {
		return d
	}
	return v
}

// stopAll detaches every sink and closes the files. Start uses it to give up
// before the watcher exists, so nobody else touches them.
func (rec *Recording) stopAll() {
	rec.mu.Lock()
	removes, writers := rec.removes, rec.writers
	rec.removes, rec.writers = nil, nil
	rec.mu.Unlock()
	for _, rm := range removes {
		rm()
	}
	for _, w := range writers {
		_ = w.Close()
	}
}

// finish waits for the track watcher and closes every file: without that
// wait a late track could be docked into a file that is already closed.
func (rec *Recording) finish() {
	close(rec.stop)
	<-rec.done
	rec.stopAll()
}

// Stop ends a recording and, when ffmpeg exists, muxes the tracks into one .mkv.
func (r *Recorder) Stop(room, stream string) (*Recording, error) {
	key := room + "/" + stream
	r.mu.Lock()
	rec, ok := r.active[key]
	delete(r.active, key)
	r.mu.Unlock()
	if !ok {
		return nil, errors.New("not recording")
	}
	rec.finish()
	// The IVF writers declare 640x480 because they never look at the bitstream;
	// fix the header before the muxer copies it into the .mkv.
	for _, f := range rec.Files {
		if strings.HasSuffix(f, ".ivf") {
			_ = fixIVFSize(f, r.probe())
		}
	}
	if out, err := r.mux(rec); err == nil {
		rec.Output = out
	}
	return rec, nil
}

// probe returns the ffprobe binary that sits next to the configured ffmpeg.
// Recordings are only muxed when ffmpeg exists, and ffprobe ships with it.
func (r *Recorder) probe() string {
	if r.FFmpeg == "" {
		return ""
	}
	dir, base := filepath.Split(r.FFmpeg)
	if !strings.Contains(base, "ffmpeg") {
		return ""
	}
	candidate := filepath.Join(dir, strings.Replace(base, "ffmpeg", "ffprobe", 1))
	if _, err := exec.LookPath(candidate); err != nil {
		return ""
	}
	return candidate
}

// Active lists running recordings for a room.
func (r *Recorder) Active(room string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for k, rec := range r.active {
		if strings.HasPrefix(k, room+"/") {
			out = append(out, rec.Stream)
		}
	}
	return out
}

func (r *Recorder) mux(rec *Recording) (string, error) {
	if r.FFmpeg == "" || len(rec.Files) == 0 {
		return "", errors.New("no ffmpeg")
	}
	bin, err := exec.LookPath(r.FFmpeg)
	if err != nil {
		return "", err
	}
	out := strings.TrimSuffix(rec.Files[0], filepath.Ext(rec.Files[0]))
	// Two cameras mux into <stream>-<ts>.mkv, not <stream>-<ts>-video2.mkv.
	out = recordedBase.ReplaceAllString(out, "") + ".mkv"
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	for _, f := range rec.Files {
		args = append(args, "-i", f)
	}
	for i := range rec.Files {
		args = append(args, "-map", fmt.Sprint(i))
	}
	args = append(args, "-c", "copy", out)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := exec.CommandContext(ctx, bin, args...).Run(); err != nil {
		return "", err
	}
	return out, nil
}
