// Package sfu is a small Selective Forwarding Unit built on pion/webrtc.
//
// A publisher uploads its media once; the server fans every RTP packet out to
// any number of sinks: WebRTC subscribers (browsers, OBS browser sources,
// WHEP players), recorders and the RTMP restreamer. Nothing is decoded, so
// latency stays at network level and CPU stays low.
package sfu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// Options configures the media engine.
type Options struct {
	RecordTraffic    func(int)
	ICEServers       []webrtc.ICEServer
	ICEConfiguration func() webrtc.Configuration
	// LocalICEConfiguration is an operator-defined alternative, never supplied by a client.
	LocalICEConfiguration func() webrtc.Configuration
	PublicIPs             []string
	UDPPort               int
	UDPPortMin            uint16
	UDPPortMax            uint16
	// GatherTimeout bounds how long we wait for ICE gathering when building
	// non-trickle SDP (WHIP/WHEP style).
	GatherTimeout time.Duration
	Logger        *slog.Logger
}

// Engine creates peer connections that share one configured pion API.
type Engine struct {
	api    *webrtc.API
	opts   Options
	log    *slog.Logger
	udpMux ice.UDPMux
	closer io.Closer
}

// NewEngine builds the pion API with codecs, interceptors and network settings.
func NewEngine(o Options) (*Engine, error) {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.GatherTimeout == 0 {
		o.GatherTimeout = 3 * time.Second
	}
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, fmt.Errorf("codecs: %w", err)
	}
	ir := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, ir); err != nil {
		return nil, fmt.Errorf("interceptors: %w", err)
	}
	// Ask publishers for a keyframe every 3s as a safety net for late joiners
	// and lossy links; explicit PLIs are still forwarded immediately.
	pli, err := intervalpli.NewReceiverInterceptor(intervalpli.GeneratorInterval(3 * time.Second))
	if err != nil {
		return nil, err
	}
	ir.Add(pli)

	se := webrtc.SettingEngine{}
	se.SetIncludeLoopbackCandidate(true)
	if len(o.PublicIPs) > 0 {
		se.SetNAT1To1IPs(o.PublicIPs, webrtc.ICECandidateTypeHost)
	}
	e := &Engine{opts: o, log: o.Logger}
	if o.UDPPort > 0 {
		conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: o.UDPPort})
		if err != nil {
			return nil, fmt.Errorf("udp mux listen :%d: %w", o.UDPPort, err)
		}
		mux := webrtc.NewICEUDPMux(nil, conn)
		se.SetICEUDPMux(mux)
		e.udpMux, e.closer = mux, conn
	} else if o.UDPPortMin > 0 {
		if err := se.SetEphemeralUDPPortRange(o.UDPPortMin, o.UDPPortMax); err != nil {
			return nil, err
		}
	}
	e.api = webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(ir), webrtc.WithSettingEngine(se))
	return e, nil
}

// Close releases shared sockets.
func (e *Engine) Close() error {
	if e.udpMux != nil {
		_ = e.udpMux.Close()
	}
	if e.closer != nil {
		return e.closer.Close()
	}
	return nil
}

type localRelayContextKey struct{}

// WithLocalRelay chooses the installed relay for one authenticated socket.
func WithLocalRelay(ctx context.Context) context.Context {
	return context.WithValue(ctx, localRelayContextKey{}, true)
}

func (e *Engine) trafficRecorder(ctx context.Context) func(int) {
	if local, _ := ctx.Value(localRelayContextKey{}).(bool); local {
		return nil
	}
	return e.opts.RecordTraffic
}

func (e *Engine) newPC(ctx context.Context) (*webrtc.PeerConnection, error) {
	configuration := webrtc.Configuration{ICEServers: e.opts.ICEServers}
	if local, _ := ctx.Value(localRelayContextKey{}).(bool); local {
		if e.opts.LocalICEConfiguration == nil {
			return nil, errors.New("local relay is not configured")
		}
		configuration = e.opts.LocalICEConfiguration()
	} else if e.opts.ICEConfiguration != nil {
		configuration = e.opts.ICEConfiguration()
	}
	configuration.BundlePolicy = webrtc.BundlePolicyMaxBundle
	configuration.SDPSemantics = webrtc.SDPSemanticsUnifiedPlan
	return e.api.NewPeerConnection(configuration)
}

// complete waits (bounded) for ICE gathering so the SDP carries candidates.
func (e *Engine) complete(ctx context.Context, pc *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
	done := webrtc.GatheringCompletePromise(pc)
	t := time.NewTimer(e.opts.GatherTimeout)
	defer t.Stop()
	select {
	case <-done:
	case <-t.C:
	case <-ctx.Done():
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return pc.LocalDescription(), nil
}

// WebSocket peers exchange candidates as they arrive, including slow TURN
// allocations. HTTP WHIP/WHEP clients retain the non-trickle SDP path.
func trickle(pc *webrtc.PeerConnection, callbacks []func(webrtc.ICECandidateInit)) {
	if len(callbacks) == 0 {
		return
	}
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			callbacks[0](c.ToJSON())
		}
	})
}

func (e *Engine) description(ctx context.Context, pc *webrtc.PeerConnection, incremental bool) (*webrtc.SessionDescription, error) {
	if !incremental {
		return e.complete(ctx, pc)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return pc.LocalDescription(), nil
}

// Sink receives forwarded RTP. TrackLocalStaticRTP, pion media writers and
// UDP pipes all satisfy it.
type Sink interface {
	WriteRTP(*rtp.Packet) error
}

type meteredSink struct {
	Sink
	record func(int)
}

func (s *meteredSink) WriteRTP(p *rtp.Packet) error {
	err := s.Sink.WriteRTP(p)
	if err == nil {
		s.record(p.MarshalSize())
	}
	return err
}

// Queue depths: realtime sinks stay shallow so a slow consumer adds at most
// ~2.5s of latency at 100 pkt/s before dropping; reliable sinks (recording)
// get a deep buffer so a disk hiccup never loses media.
const (
	realtimeQueueDepth = 256
	reliableQueueDepth = 8192
)

// sinkQueue decouples the publisher read loop from one sink's writes: a
// bounded channel plus a dedicated writer goroutine. A slow realtime sink
// (congested viewer, TURN relay) can no longer stall the fan-out and distort
// the arrival times the TWCC generator reports to the publisher.
type sinkQueue struct {
	w        Sink
	ch       chan *rtp.Packet
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	reliable bool
	log      *slog.Logger
	desc     string

	// mu guards stopped/closing/inflight and handshakes a reliable push (which
	// may block on a full queue) with the flush. closing stops admitting new
	// pushes; stopped means everything accepted has been written.
	mu       sync.Mutex
	stopped  bool
	closing  bool
	inflight int
	wake     chan struct{}

	enqueued  atomic.Int64 // reliable packets accepted into ch (tests)
	writeErrs atomic.Int64 // failed WriteRTP calls; the first one is also logged
	fullWarn  atomic.Bool
}

func newSinkQueue(w Sink, reliable bool, log *slog.Logger, desc string) *sinkQueue {
	depth := realtimeQueueDepth
	if reliable {
		depth = reliableQueueDepth
	}
	if log == nil {
		log = slog.Default()
	}
	sq := &sinkQueue{
		w:        w,
		ch:       make(chan *rtp.Packet, depth),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		wake:     make(chan struct{}, 1),
		reliable: reliable,
		log:      log,
		desc:     desc,
	}
	go sq.writeLoop()
	return sq
}

func (s *sinkQueue) writeLoop() {
	defer close(s.done)
	for {
		select {
		case p := <-s.ch:
			s.write(p)
		case <-s.stop:
			if s.reliable {
				s.flush()
			}
			return
		}
	}
}

// drain writes everything currently buffered, without blocking.
func (s *sinkQueue) drain() {
	for {
		select {
		case p := <-s.ch:
			s.write(p)
		default:
			return
		}
	}
}

// flush drains the queue of a reliable sink before it is declared closed. It
// first stops admitting new pushes, then waits for the pushes already admitted
// (they may be blocked on a full queue, so it keeps servicing the channel),
// and only then closes: every packet a push accepted is written before done is
// closed, and nothing can be enqueued after the final drain.
func (s *sinkQueue) flush() {
	s.mu.Lock()
	s.closing = true
	s.mu.Unlock()

	for {
		s.mu.Lock()
		n := s.inflight
		s.mu.Unlock()
		if n == 0 {
			break
		}
		// Free room for a blocked sender (and write what is already queued).
		select {
		case p := <-s.ch:
			s.write(p)
		case <-s.wake:
		case <-time.After(time.Millisecond):
		}
	}

	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
	s.drain()
}

// write hands one packet to the sink. Failures are logged once per sink (a
// broken disk must not stay invisible) and only counted afterwards; a closed
// pipe is a normal viewer disconnect, not a failure worth a log line.
func (s *sinkQueue) write(p *rtp.Packet) {
	err := s.w.WriteRTP(p)
	if err == nil || errors.Is(err, io.ErrClosedPipe) {
		return
	}
	if n := s.writeErrs.Add(1); n == 1 {
		s.log.Warn("sink write failed; further failures are only counted", "sink", s.desc, "err", err)
	}
}

// push hands one packet to the writer. Realtime: drop-oldest on overflow
// (losing a stale packet beats starving every other consumer). Reliable: wait
// for a slot — never drop; the deep queue makes this rare, and on the rare
// full queue the whole fan-out deliberately waits instead of losing recorded
// media (a hung disk stalls the fan-out rather than truncating the file).
func (s *sinkQueue) push(p *rtp.Packet) {
	if s.reliable {
		s.mu.Lock()
		if s.stopped || s.closing {
			s.mu.Unlock()
			return
		}
		s.inflight++
		s.mu.Unlock()

		select {
		case s.ch <- p:
		default:
			if s.fullWarn.CompareAndSwap(false, true) {
				s.log.Warn("reliable sink queue full; the fan-out waits for it to drain", "sink", s.desc, "depth", cap(s.ch))
			}
			s.ch <- p
		}
		s.enqueued.Add(1)

		s.mu.Lock()
		s.inflight--
		s.mu.Unlock()
		select {
		case s.wake <- struct{}{}:
		default:
		}
		return
	}
	select {
	case s.ch <- p:
		return
	case <-s.stop:
		return
	default:
	}
	// Full: drop the oldest. The read loop is the only producer, so after
	// freeing one slot the send below cannot block.
	select {
	case <-s.ch:
	default:
	}
	select {
	case s.ch <- p:
	case <-s.stop:
	}
}

func (s *sinkQueue) shutdown() { s.stopOnce.Do(func() { close(s.stop) }) }

// Forwarder fans one incoming track out to many sinks.
type Forwarder struct {
	recordTraffic func(int)
	log           *slog.Logger
	ID            string
	StreamID      string
	Kind          webrtc.RTPCodecType
	Codec         webrtc.RTPCodecCapability
	PT            uint8

	remote *webrtc.TrackRemote
	pc     *webrtc.PeerConnection

	mu      sync.RWMutex
	sinks   map[uint64]*sinkQueue
	nextID  uint64
	packets atomic.Uint64
	bytes   atomic.Uint64
}

// AddSink registers a realtime sink (viewers, restream) and returns a
// function that removes it. When the sink falls behind, its oldest queued
// packets are dropped so the rest of the fan-out keeps flowing.
func (f *Forwarder) AddSink(s Sink) (remove func()) { return f.addSink(s, false) }

// AddReliableSink registers a sink that must receive every packet (server
// recording). It writes in its own goroutine with a deep buffer and never
// drops; removal flushes the queue before returning, so closing the file
// afterwards cannot truncate the recording.
func (f *Forwarder) AddReliableSink(s Sink) (remove func()) { return f.addSink(s, true) }

func (f *Forwarder) addSink(s Sink, reliable bool) (remove func()) {
	sq := newSinkQueue(s, reliable, f.log, fmt.Sprintf("%s/%s", f.StreamID, f.ID))
	f.mu.Lock()
	f.nextID++
	id := f.nextID
	if f.sinks == nil {
		f.sinks = map[uint64]*sinkQueue{}
	}
	f.sinks[id] = sq
	f.mu.Unlock()
	f.RequestKeyframe()
	return func() {
		f.mu.Lock()
		entry, ok := f.sinks[id]
		delete(f.sinks, id)
		f.mu.Unlock()
		if ok {
			entry.shutdown()
			// A realtime writer returns immediately; a reliable one flushes
			// its queue before done closes, so the caller can close the file
			// without truncating the recording.
			<-entry.done
		}
	}
}

// SinkCount returns the number of attached sinks.
func (f *Forwarder) SinkCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.sinks)
}

// RequestKeyframe sends a PLI upstream so new viewers get a picture fast.
func (f *Forwarder) RequestKeyframe() {
	if f.Kind != webrtc.RTPCodecTypeVideo || f.pc == nil || f.remote == nil {
		return
	}
	_ = f.pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(f.remote.SSRC())}})
}

// Stats returns forwarded packet and byte counters.
func (f *Forwarder) Stats() (packets, bytes uint64) { return f.packets.Load(), f.bytes.Load() }

func (f *Forwarder) run(log *slog.Logger) {
	buf := make([]byte, 1500)
	pkt := &rtp.Packet{}
	var snapshot []*sinkQueue
	// When the read loop ends, tell every sink writer to stop so no goroutine
	// outlives the forwarder. This does not wait: remove() is what waits for a
	// reliable sink to finish flushing before the caller closes its file.
	defer func() {
		f.mu.RLock()
		for _, s := range f.sinks {
			s.shutdown()
		}
		f.mu.RUnlock()
	}()
	for {
		n, _, err := f.remote.Read(buf)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Debug("track read ended", "track", f.ID, "err", err)
			}
			return
		}
		if err := pkt.Unmarshal(buf[:n]); err != nil {
			continue
		}
		f.packets.Add(1)
		f.bytes.Add(uint64(n))
		if f.recordTraffic != nil {
			f.recordTraffic(n)
		}
		// Unmarshal aliases buf, which the next Read overwrites: deep-copy
		// before handing the packet to asynchronous sink writers.
		clone := pkt.Clone()
		f.mu.RLock()
		snapshot = snapshot[:0]
		for _, s := range f.sinks {
			snapshot = append(snapshot, s)
		}
		f.mu.RUnlock()
		for _, s := range snapshot {
			// Each sink rewrites SSRC/PT as needed, so give each its own copy
			// of the header. The payload slice stays shared: sinks only read
			// it, and every packet already has its own deep copy above.
			cp := *clone
			s.push(&cp)
		}
	}
}

// Stream is one published source (camera, screen, program, OBS WHIP push).
type Stream struct {
	ID       string
	Owner    string // peer id, "whip" for WHIP pushes
	Kind     string // camera | screen | program | whip
	Revision string // one transport instance of a stable source ID

	engine *Engine
	pc     *webrtc.PeerConnection

	mu            sync.RWMutex
	forwarders    []*Forwarder
	expected      int
	ready         chan struct{}
	readyOnce     sync.Once
	closed        chan struct{}
	closeOnce     sync.Once
	onClose       []func()
	tracksChanged chan struct{}
	created       time.Time
}

var streamSequence atomic.Uint64

// Publish accepts a publisher offer and returns the answer. expectedTracks is
// derived from the offer's m-lines so we know when the stream is complete.
func (e *Engine) Publish(ctx context.Context, id, owner, kind string, offer webrtc.SessionDescription, onICE ...func(webrtc.ICECandidateInit)) (*Stream, *webrtc.SessionDescription, error) {
	if offer.Type != webrtc.SDPTypeOffer {
		return nil, nil, errors.New("expected an SDP offer")
	}
	pc, err := e.newPC(ctx)
	if err != nil {
		return nil, nil, err
	}
	s := &Stream{ID: id, Owner: owner, Kind: kind, engine: e, pc: pc,
		Revision: fmt.Sprintf("%016x-%016x", streamSequence.Add(1), time.Now().UnixNano()), tracksChanged: make(chan struct{}, 1),
		ready: make(chan struct{}), closed: make(chan struct{}), created: time.Now(),
		expected: countMedia(offer.SDP)}
	trickle(pc, onICE)

	pc.OnTrack(func(tr *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		f := &Forwarder{
			recordTraffic: e.trafficRecorder(ctx),
			log:           e.log,
			ID:            tr.ID(), StreamID: id, Kind: tr.Kind(), Codec: tr.Codec().RTPCodecCapability,
			PT: uint8(tr.PayloadType()), remote: tr, pc: pc,
		}
		s.mu.Lock()
		s.forwarders = append(s.forwarders, f)
		n := len(s.forwarders)
		s.mu.Unlock()
		e.log.Info("track up", "stream", id, "kind", tr.Kind().String(), "codec", tr.Codec().MimeType)
		if n >= s.expected {
			s.markReady()
		}
		select {
		case s.tracksChanged <- struct{}{}:
		default:
		}
		go f.run(e.log)
	})
	pc.OnConnectionStateChange(func(st webrtc.PeerConnectionState) {
		switch st {
		case webrtc.PeerConnectionStateConnected:
			// Some publishers negotiate more m-lines than they send; don't
			// keep viewers waiting forever.
			go func() {
				select {
				case <-time.After(1500 * time.Millisecond):
					s.markReady()
				case <-s.closed:
				}
			}()
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			s.Close()
		}
	})
	if err := pc.SetRemoteDescription(offer); err != nil {
		_ = pc.Close()
		return nil, nil, fmt.Errorf("remote description: %w", err)
	}
	answer, err := pc.CreateAnswer(&webrtc.AnswerOptions{OfferAnswerOptions: webrtc.OfferAnswerOptions{ICETricklingSupported: len(onICE) > 0}})
	if err != nil {
		_ = pc.Close()
		return nil, nil, err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		_ = pc.Close()
		return nil, nil, err
	}
	desc, err := e.description(ctx, pc, len(onICE) > 0)
	if err != nil {
		s.Close()
		return nil, nil, err
	}
	return s, desc, nil
}

// countMedia counts audio/video sections the publisher will actually send.
func countMedia(sdp string) int {
	n := 0
	inMedia, sending := false, false
	flush := func() {
		if inMedia && sending {
			n++
		}
	}
	for _, l := range strings.Split(sdp, "\n") {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "m="):
			flush()
			inMedia = strings.HasPrefix(l, "m=audio") || strings.HasPrefix(l, "m=video")
			// port 0 means the section was rejected/disabled
			f := strings.Fields(l)
			sending = inMedia && len(f) > 1 && f[1] != "0"
		case l == "a=recvonly" || l == "a=inactive":
			sending = false
		}
	}
	flush()
	if n == 0 {
		n = 1
	}
	return n
}

func (s *Stream) markReady() { s.readyOnce.Do(func() { close(s.ready) }) }

// Ready is closed once the stream's tracks are flowing.
func (s *Stream) Ready() <-chan struct{} { return s.ready }

// TracksChanged signals that a late RTP track became available after Ready.
func (s *Stream) TracksChanged() <-chan struct{} { return s.tracksChanged }

// Done is closed when the stream ends.
func (s *Stream) Done() <-chan struct{} { return s.closed }

// PC exposes the publisher connection for authenticated trickle ICE signalling.
func (s *Stream) PC() *webrtc.PeerConnection { return s.pc }

// Forwarders returns a snapshot of the stream's tracks.
func (s *Stream) Forwarders() []*Forwarder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]*Forwarder(nil), s.forwarders...)
}

// OnClose registers a callback run once when the stream ends.
func (s *Stream) OnClose(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.closed:
		go fn()
	default:
		s.onClose = append(s.onClose, fn)
	}
}

// Close tears the publisher down and notifies listeners.
func (s *Stream) Close() {
	s.closeOnce.Do(func() {
		close(s.closed)
		_ = s.pc.Close()
		s.mu.Lock()
		cbs := s.onClose
		s.onClose = nil
		s.mu.Unlock()
		for _, fn := range cbs {
			fn()
		}
	})
}

// Info is a JSON friendly description of a stream.
type Info struct {
	ID       string   `json:"id"`
	Owner    string   `json:"owner"`
	Kind     string   `json:"kind"`
	Revision string   `json:"revision,omitempty"`
	Tracks   []string `json:"tracks"`
	Viewers  int      `json:"viewers"`
	Since    int64    `json:"since"`
}

// Info describes the stream.
func (s *Stream) Info() Info {
	inf := Info{ID: s.ID, Owner: s.Owner, Kind: s.Kind, Revision: s.Revision, Since: s.created.Unix()}
	for _, f := range s.Forwarders() {
		inf.Tracks = append(inf.Tracks, f.Kind.String()+":"+f.Codec.MimeType)
		if c := f.SinkCount(); c > inf.Viewers {
			inf.Viewers = c
		}
	}
	return inf
}

// Subscriber is a send-only peer connection receiving one stream.
type Subscriber struct {
	pc      *webrtc.PeerConnection
	removes []func()
	once    sync.Once
	done    chan struct{}
}

// Close detaches the subscriber.
func (sub *Subscriber) Close() {
	sub.once.Do(func() {
		for _, r := range sub.removes {
			r()
		}
		_ = sub.pc.Close()
		close(sub.done)
	})
}

// Done is closed when the subscriber goes away.
func (sub *Subscriber) Done() <-chan struct{} { return sub.done }

// PC exposes the underlying peer connection (for answer handling).
func (sub *Subscriber) PC() *webrtc.PeerConnection { return sub.pc }

func (e *Engine) attach(ctx context.Context, s *Stream, pc *webrtc.PeerConnection) (*Subscriber, error) {
	sub := &Subscriber{pc: pc, done: make(chan struct{})}
	for _, f := range s.Forwarders() {
		local, err := webrtc.NewTrackLocalStaticRTP(f.Codec, f.ID, s.ID)
		if err != nil {
			return nil, err
		}
		sender, err := pc.AddTrack(local)
		if err != nil {
			return nil, err
		}
		ff := f
		go func() {
			buf := make([]byte, 1500)
			for {
				n, _, err := sender.Read(buf)
				if err != nil {
					return
				}
				pkts, err := rtcp.Unmarshal(buf[:n])
				if err != nil {
					continue
				}
				for _, p := range pkts {
					switch p.(type) {
					case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
						ff.RequestKeyframe()
					}
				}
			}
		}()
		var sink Sink = local
		if record := e.trafficRecorder(ctx); record != nil {
			sink = &meteredSink{Sink: local, record: record}
		}
		sub.removes = append(sub.removes, f.AddSink(sink))
	}
	pc.OnConnectionStateChange(func(st webrtc.PeerConnectionState) {
		if st == webrtc.PeerConnectionStateConnected {
			// AddSink's initial PLI precedes ICE/DTLS. Ask again only once the
			// receiver can accept RTP instead of waiting for the 3s safety PLI.
			for _, f := range s.Forwarders() {
				f.RequestKeyframe()
			}
		}
		if st == webrtc.PeerConnectionStateFailed || st == webrtc.PeerConnectionStateClosed {
			sub.Close()
		}
	})
	s.OnClose(sub.Close)
	return sub, nil
}

// SubscribeOffer creates a server-initiated subscription (used over the
// WebSocket signalling): the server sends an offer, the client answers.
func (e *Engine) SubscribeOffer(ctx context.Context, s *Stream, onICE ...func(webrtc.ICECandidateInit)) (*Subscriber, *webrtc.SessionDescription, error) {
	if err := waitReady(ctx, s); err != nil {
		return nil, nil, err
	}
	pc, err := e.newPC(ctx)
	if err != nil {
		return nil, nil, err
	}
	sub, err := e.attach(ctx, s, pc)
	if err != nil {
		_ = pc.Close()
		return nil, nil, err
	}
	trickle(pc, onICE)
	offer, err := pc.CreateOffer(&webrtc.OfferOptions{OfferAnswerOptions: webrtc.OfferAnswerOptions{ICETricklingSupported: len(onICE) > 0}})
	if err != nil {
		sub.Close()
		return nil, nil, err
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		sub.Close()
		return nil, nil, err
	}
	desc, err := e.description(ctx, pc, len(onICE) > 0)
	if err != nil {
		sub.Close()
		return nil, nil, err
	}
	return sub, desc, nil
}

// SubscribeAnswer handles client-initiated subscriptions (WHEP): the client
// sends a recvonly offer and receives an answer.
func (e *Engine) SubscribeAnswer(ctx context.Context, s *Stream, offer webrtc.SessionDescription) (*Subscriber, *webrtc.SessionDescription, error) {
	if err := waitReady(ctx, s); err != nil {
		return nil, nil, err
	}
	pc, err := e.newPC(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := pc.SetRemoteDescription(offer); err != nil {
		_ = pc.Close()
		return nil, nil, fmt.Errorf("remote description: %w", err)
	}
	sub, err := e.attach(ctx, s, pc)
	if err != nil {
		_ = pc.Close()
		return nil, nil, err
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		sub.Close()
		return nil, nil, err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		sub.Close()
		return nil, nil, err
	}
	desc, err := e.complete(ctx, pc)
	if err != nil {
		sub.Close()
		return nil, nil, err
	}
	return sub, desc, nil
}

func waitReady(ctx context.Context, s *Stream) error {
	select {
	case <-s.Ready():
		return nil
	case <-s.Done():
		return errors.New("stream ended")
	case <-ctx.Done():
		return fmt.Errorf("stream %q not ready: %w", s.ID, ctx.Err())
	}
}
