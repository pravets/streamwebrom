package sfu

// Tests for ticket #16 (guest stuck at ~101 kbps, pixelated stream).
//
// TestPublisherReceivesRTCPFeedback: the SFU must send RTCP congestion
// feedback (RR / transport-cc) back to the publisher; Chrome's send-side BWE
// needs it to ramp the bitrate up.
//
// TestSlowSinkMustNotStarveForwarder: regression test for the asynchronous
// fan-out fix — one slow realtime sink must not stall the publisher read loop
// (which distorted TWCC arrival times and collapsed delay-based BWE).
//
// TestReliableSinkGetsEveryPacket: a reliable sink (recording) must receive
// every packet in order even while realtime sinks overflow and drop.

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// rtcpSniffer records every RTCP packet the publisher receives from the SFU.
type rtcpSniffer struct {
	interceptor.NoOp
	mu   sync.Mutex
	seen []rtcp.Packet
}

func (s *rtcpSniffer) BindRTCPReader(reader interceptor.RTCPReader) interceptor.RTCPReader {
	return interceptor.RTCPReaderFunc(func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
		n, attr, err := reader.Read(b, a)
		if err == nil && n > 0 {
			if pkts, e := rtcp.Unmarshal(b[:n]); e == nil {
				s.mu.Lock()
				s.seen = append(s.seen, pkts...)
				s.mu.Unlock()
			}
		}
		return n, attr, err
	})
}

func (s *rtcpSniffer) snapshot() []rtcp.Packet {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]rtcp.Packet(nil), s.seen...)
}

// snifferFactory adapts the sniffer to interceptor.Factory for Registry.Add.
type snifferFactory struct{ s *rtcpSniffer }

func (f snifferFactory) NewInterceptor(_ string) (interceptor.Interceptor, error) {
	return f.s, nil
}

// pumpCtl controls the test publisher's media pump.
type pumpCtl struct {
	cancel context.CancelFunc
	paused atomic.Bool
}

func (p *pumpCtl) stop()       { p.cancel() }
func (p *pumpCtl) pause(v bool) { p.paused.Store(v) }

// pumpFeedback mirrors internal/server's pump: a real pion track carrying a
// minimal VP8 keyframe-looking payload at ~100 pkt/s; the SFU never decodes it.
func pumpFeedback(ctx context.Context, track *webrtc.TrackLocalStaticRTP, paused *atomic.Bool) {
	seq := uint16(0)
	ts := uint32(0)
	payload := append([]byte{0x10, 0x00, 0x00, 0x9d, 0x01, 0x2a}, bytes.Repeat([]byte{0xAB}, 400)...)
	t := time.NewTicker(10 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if paused.Load() {
				continue
			}
			seq++
			ts += 3000
			_ = track.WriteRTP(&rtp.Packet{Header: rtp.Header{Version: 2, Marker: true, SequenceNumber: seq, Timestamp: ts, PayloadType: 96}, Payload: payload})
		}
	}
}

// startPublisher publishes a real VP8 track into the engine the way a browser
// does (default codecs + TWCC header extension on outgoing RTP) and returns
// the connected publisher, its track, the stream and an RTCP sniffer.
func startPublisher(t *testing.T, e *Engine, id string) (*webrtc.PeerConnection, *webrtc.TrackLocalStaticRTP, *Stream, *rtcpSniffer, string, *pumpCtl) {
	t.Helper()
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		t.Fatal(err)
	}
	ir := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, ir); err != nil {
		t.Fatal(err)
	}
	if err := webrtc.ConfigureTWCCHeaderExtensionSender(m, ir); err != nil {
		t.Fatal(err)
	}
	sniff := &rtcpSniffer{}
	ir.Add(snifferFactory{sniff})
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(ir))
	pub, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pub.Close() })

	track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}, "video", id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pub.AddTransceiverFromTrack(track, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); err != nil {
		t.Fatal(err)
	}
	offer, err := pub.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	<-webrtc.GatheringCompletePromise(pub)

	stream, answer, err := e.Publish(context.Background(), id, "test", "camera", *pub.LocalDescription())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stream.Close)
	if err := pub.SetRemoteDescription(*answer); err != nil {
		t.Fatal(err)
	}

	// pion delivers incoming RTCP through the interceptor chain only when
	// someone reads it (browsers do this internally); drive the sender's
	// ReadRTCP so the sniffer sees what the SFU actually sends back.
	go func() {
		senders := pub.GetSenders()
		if len(senders) == 0 {
			return
		}
		for {
			if _, _, err := senders[0].ReadRTCP(); err != nil {
				return
			}
		}
	}()

	// Start media immediately: pion fires OnTrack (and creates the forwarder)
	// on the first RTP packet, so the pump must run before we wait for it.
	ctx, cancel := context.WithCancel(context.Background())
	ctl := &pumpCtl{cancel: cancel}
	t.Cleanup(cancel)
	go pumpFeedback(ctx, track, &ctl.paused)

	deadline := time.After(5 * time.Second)
	for pub.ConnectionState() != webrtc.PeerConnectionStateConnected {
		select {
		case <-deadline:
			t.Fatalf("publisher never connected: %s", pub.ConnectionState())
		case <-time.After(50 * time.Millisecond):
		}
	}
	fwdDeadline := time.After(5 * time.Second)
	for len(stream.Forwarders()) == 0 {
		select {
		case <-fwdDeadline:
			t.Fatal("no forwarders appeared")
		case <-time.After(50 * time.Millisecond):
		}
	}
	return pub, track, stream, sniff, answer.SDP, ctl
}

// TestPublisherReceivesRTCPFeedback asserts that within ~5 seconds the
// publisher receives congestion feedback (RR and transport-cc), and that the
// answer SDP negotiates transport-cc (without it Chrome never marks its RTP
// for TWCC and delay-based BWE is blind).
func TestPublisherReceivesRTCPFeedback(t *testing.T) {
	e, err := NewEngine(Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_, _, stream, sniff, answerSDP, _ := startPublisher(t, e, "cam")

	// SDP fact: transport-cc must be negotiated in the answer, otherwise the
	// browser sender will not attach the TWCC header extension.
	hasTWCCExt := strings.Contains(answerSDP, "transport-cc")
	t.Logf("answer SDP negotiates transport-cc: %v", hasTWCCExt)
	if !hasTWCCExt {
		t.Errorf("answer SDP does not negotiate transport-cc; Chrome BWE loses delay-based feedback")
	}
	fs := stream.Forwarders()
	if len(fs) == 0 {
		t.Fatal("no forwarders")
	}

	deadline := time.After(5 * time.Second)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	var rr, twcc, sr, other int
	for {
		select {
		case <-deadline:
			pkts, byts := fs[0].Stats()
			t.Logf("SFU forwarder received: packets=%d bytes=%d", pkts, byts)
			for _, p := range sniff.snapshot() {
				switch p.(type) {
				case *rtcp.ReceiverReport:
					rr++
				case *rtcp.TransportLayerCC:
					twcc++
				case *rtcp.SenderReport:
					sr++
				default:
					other++
				}
			}
			t.Logf("RTCP received by publisher in 5s: RR=%d TWCC=%d SR=%d other=%d", rr, twcc, sr, other)
			if rr == 0 && twcc == 0 {
				t.Fatalf("RED: publisher received no ReceiverReport and no TransportLayerCC in 5s (RR=%d TWCC=%d SR=%d other=%d); send-side BWE is blind", rr, twcc, sr, other)
			}
			if twcc == 0 {
				t.Errorf("no transport-cc feedback in 5s (RR=%d): Chrome delay-based BWE cannot ramp up", rr)
			}
			return
		case <-tick.C:
		}
	}
}

// stallSink simulates a slow consumer: congested subscriber, TURN relay, disk.
type stallSink struct {
	delay time.Duration
	n     atomic.Int64
}

func (s *stallSink) WriteRTP(*rtp.Packet) error {
	time.Sleep(s.delay)
	s.n.Add(1)
	return nil
}

// maxRecvDelta extracts the largest receive-delta (in microseconds; pion
// stores RecvDelta.Delta already scaled by 250) from TWCC feedback packets:
// nominal ~10ms pump => ~10000; a stalled forwarder => tens of ms.
func maxRecvDelta(pkts []rtcp.Packet) int64 {
	var max int64
	for _, p := range pkts {
		twcc, ok := p.(*rtcp.TransportLayerCC)
		if !ok {
			continue
		}
		for _, d := range twcc.RecvDeltas {
			if d.Delta > max {
				max = d.Delta
			}
		}
	}
	return max
}

// TestSlowSinkMustNotStarveForwarder is the regression test for the fan-out
// fix: one slow realtime sink must not throttle the publisher read loop. The
// pump offers 100 pkt/s; even with a 50ms-per-packet stall sink the forwarder
// must keep processing ~100 pkt/s (drop-oldest in the slow sink's queue).
func TestSlowSinkMustNotStarveForwarder(t *testing.T) {
	e, err := NewEngine(Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_, _, stream, sniff, _, _ := startPublisher(t, e, "cam")
	fs := stream.Forwarders()
	if len(fs) == 0 {
		t.Fatal("no forwarders")
	}

	sink := &stallSink{delay: 50 * time.Millisecond}
	remove := fs[0].AddSink(sink)
	defer remove()

	before, _ := fs[0].Stats()
	time.Sleep(2 * time.Second)
	after, _ := fs[0].Stats()
	processed := after - before
	delta := maxRecvDelta(sniff.snapshot())
	t.Logf("with one 50ms-stall sink: forwarder processed %d packets in 2s (pump=100/s, want ~200); sink writes=%d; max TWCC recv delta=%dus (=%dms, nominal ~10ms)",
		processed, sink.n.Load(), delta, delta/1000)
	if processed < 150 {
		t.Fatalf("one slow sink throttled the forwarder to %d pkt/2s (want >=150): publisher TWCC arrival times distort and delay-based BWE collapses", processed)
	}
}

// seqSink counts packets and verifies they arrive in order without gaps.
type seqSink struct {
	n       atomic.Int64
	mu      sync.Mutex
	lastSeq uint16
	started bool
	gaps    atomic.Int64
}

func (c *seqSink) WriteRTP(p *rtp.Packet) error {
	c.mu.Lock()
	if c.started && p.SequenceNumber != c.lastSeq+1 {
		c.gaps.Add(1)
	}
	c.lastSeq = p.SequenceNumber
	c.started = true
	c.mu.Unlock()
	c.n.Add(1)
	return nil
}

// TestReliableSinkGetsEveryPacket asserts that a reliable sink (recording)
// receives EVERY packet in order, even while a realtime sink is stalled and
// its queue overflows with drops.
func TestReliableSinkGetsEveryPacket(t *testing.T) {
	e, err := NewEngine(Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_, _, stream, _, _, pump := startPublisher(t, e, "cam")
	fs := stream.Forwarders()
	if len(fs) == 0 {
		t.Fatal("no forwarders")
	}

	// Pause the pump and let in-flight packets drain, so the baseline is
	// exact: every packet counted after it must reach the reliable sink.
	pump.pause(true)
	time.Sleep(200 * time.Millisecond)
	base, _ := fs[0].Stats()

	reliable := &seqSink{}
	removeReliable := fs[0].AddReliableSink(reliable)
	defer removeReliable()
	slow := &stallSink{delay: 100 * time.Millisecond}
	removeSlow := fs[0].AddSink(slow)
	defer removeSlow()

	pump.pause(false)
	// 3s at 100 pkt/s = ~300 packets; the 100ms-stall realtime sink writes
	// ~30, so its 256-slot queue overflows and drops oldest.
	time.Sleep(3 * time.Second)
	pump.stop()
	time.Sleep(300 * time.Millisecond) // let in-flight packets reach the forwarder

	want, _ := fs[0].Stats()
	want -= base
	deadline := time.After(5 * time.Second)
	for reliable.n.Load() < int64(want) {
		select {
		case <-deadline:
			t.Fatalf("reliable sink got %d of %d packets", reliable.n.Load(), want)
		case <-time.After(50 * time.Millisecond):
		}
	}
	// Removal flushes the reliable queue; after it the count must be exact.
	removeReliable()
	got := reliable.n.Load()
	t.Logf("reliable sink: %d/%d packets, gaps=%d; stalled realtime sink writes=%d", got, want, reliable.gaps.Load(), slow.n.Load())
	if got != int64(want) {
		t.Fatalf("reliable sink got %d packets, want exactly %d (no losses, no duplicates)", got, want)
	}
	if reliable.gaps.Load() != 0 {
		t.Fatalf("reliable sink saw %d sequence gaps: order/continuity broken", reliable.gaps.Load())
	}
	if slow.n.Load() >= int64(want) {
		t.Logf("note: realtime sink kept up (%d writes); drop path not exercised this run", slow.n.Load())
	}
}