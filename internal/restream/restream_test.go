package restream

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"
)

func TestReservedRTPPorts(t *testing.T) {
	sockets, port, err := reserveRTPPorts()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, s := range sockets {
			_ = s.Close()
		}
	}()
	if port%2 != 0 {
		t.Fatal("RTP port must be even")
	}
	for _, p := range []int{port, port + 1} {
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: p})
		if err == nil {
			conn.Close()
			t.Fatalf("port %d not reserved", p)
		}
	}
}

func TestStatsNeverExposeDestinationKeys(t *testing.T) {
	s := &Session{}
	s.readStats(strings.NewReader("[tee] Unable to open rtmp://example/live/secret-key\nfps=30\nbitrate=4000k\nout_time=00:00:05\nprogress=continue\n"))
	status := s.Status()
	if strings.Contains(status.LastError, "secret-key") || status.Stats != "30 fps · 4000k · 00:00:05" {
		t.Fatalf("unsafe or invalid status: %+v", status)
	}
}

func TestScrubSecrets(t *testing.T) {
	cases := []struct{ in, want string }{
		{"[tee] Unable to open rtmp://a.example/live/secret-key", "[tee] Unable to open rtmp://***"},
		{"rtmps://b.example/app/key?password=hunter2 failed", "rtmps://*** failed"},
		{"srt://c.example:9000?streamid=abc&passphrase=xyz", "srt://***"},
		{"Output url key=abc123 rejected", "Output url key=*** rejected"},
		{"Non-monotonous DTS in output stream 0:1", "Non-monotonous DTS in output stream 0:1"},
	}
	for _, c := range cases {
		if got := scrubSecrets(c.in); got != c.want {
			t.Fatalf("scrubSecrets(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestReadStatsClearsStaleErrorWhenProgressResumes(t *testing.T) {
	s := &Session{}
	s.readStats(strings.NewReader("Non-monotonous DTS in output stream 0:1\nfps=14.26\nbitrate=N/A\nout_time=00:15:55.157333\nprogress=continue\n"))
	st := s.Status()
	if st.LastError != "" {
		t.Fatalf("stale error pinned: %q", st.LastError)
	}
	if st.Stats != "14.26 fps · N/A · 00:15:55.157333" {
		t.Fatalf("stats: %q", st.Stats)
	}
}

func TestReadStatsLogsScrubbedCause(t *testing.T) {
	var buf bytes.Buffer
	s := &Session{log: slog.New(slog.NewTextHandler(&buf, nil)), stream: "program"}
	s.readStats(strings.NewReader("[tee] Unable to open rtmp://example/live/secret-key\n"))
	st := s.Status()
	if st.LastError == "" || strings.Contains(st.LastError, "secret-key") {
		t.Fatalf("indicator: %q", st.LastError)
	}
	out := buf.String()
	if !strings.Contains(out, "ffmpeg stderr") || !strings.Contains(out, "rtmp://***") || strings.Contains(out, "secret-key") {
		t.Fatalf("journal must carry the scrubbed cause: %q", out)
	}
}

func TestDestinationValidate(t *testing.T) {
	ok := []string{"rtmp://a.rtmp.youtube.com/live2/abcd", "rtmps://live-api-s.facebook.com:443/rtmp/k", "srt://host:9000?streamid=x"}
	bad := []string{"file:///etc/passwd", "concat:a|b", "rtmp://", "http://x/y", "rtmp://h/app/key|[f=flv]file.flv", "pipe:1"}
	for _, u := range ok {
		if err := (Destination{Name: "x", URL: u}).Validate(); err != nil {
			t.Errorf("%s should be valid: %v", u, err)
		}
	}
	for _, u := range bad {
		if err := (Destination{Name: "x", URL: u}).Validate(); err == nil {
			t.Errorf("%s should be rejected", u)
		}
	}
}

func TestBuildArgs(t *testing.T) {
	d := []Destination{{Name: "yt", URL: "rtmp://a/live2/k1"}, {Name: "srt", URL: "srt://h:1"}}
	a := strings.Join(BuildArgs("video/H264", d, Settings{}), " ")
	if !strings.Contains(a, "-c:v copy") {
		t.Error("H264 input must be copied, not transcoded")
	}
	if !strings.Contains(a, "[f=flv:onfail=ignore]rtmp://a/live2/k1|[f=mpegts:onfail=ignore]srt://h:1") {
		t.Errorf("tee output wrong: %s", a)
	}
	v := strings.Join(BuildArgs("video/VP8", d, Settings{VideoKbps: 3000, FPS: 60}), " ")
	if !strings.Contains(v, "libx264") || !strings.Contains(v, "-b:v 3000k") || !strings.Contains(v, "-g 120") {
		t.Errorf("VP8 must be transcoded with settings: %s", v)
	}
	if !strings.Contains(v, "-protocol_whitelist pipe,udp,rtp") {
		t.Error("ffmpeg input protocols must be whitelisted")
	}
}

func TestBuildSDP(t *testing.T) {
	s := BuildSDP([]TrackDesc{
		{Audio: true, Port: 5000, PT: 111, Mime: "audio/opus", ClockRate: 48000, Channels: 2, Fmtp: "minptime=10;useinbandfec=1"},
		{Port: 5002, PT: 96, Mime: "video/VP8", ClockRate: 90000},
	})
	for _, want := range []string{"m=audio 5000 RTP/AVP 111", "a=rtpmap:111 opus/48000/2", "a=fmtp:111 minptime=10", "m=video 5002 RTP/AVP 96", "a=rtpmap:96 VP8/90000"} {
		if !strings.Contains(s, want) {
			t.Errorf("sdp missing %q:\n%s", want, s)
		}
	}
}
