package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// dmFixture wires a room with one host and two guests directly into the
// structs, like screenFixture: no sockets, messages are read from peer.out.
func dmFixture(t *testing.T) (*Hub, *Room, *Peer, *Peer, *Peer) {
	t.Helper()
	r, _, err := newRoom("dm-test", "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	host, g1, g2 := newPeer(nil, nil), newPeer(nil, nil), newPeer(nil, nil)
	host.ID, host.Role, host.room = "host", RoleHost, r
	g1.ID, g1.Role, g1.room = "guest-one", RoleGuest, r
	g2.ID, g2.Role, g2.room = "guest-two", RoleGuest, r
	host.Name, g1.Name, g2.Name = "Host", "Ana", "Bob"
	r.peers[host.ID], r.peers[g1.ID], r.peers[g2.ID] = host, g1, g2
	return &Hub{}, r, host, g1, g2
}

func dmRead(t *testing.T, p *Peer) Msg {
	t.Helper()
	select {
	case b := <-p.out:
		var m Msg
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for a message on %s", p.ID)
		return Msg{}
	}
}

func dmQuiet(t *testing.T, p *Peer) {
	t.Helper()
	select {
	case b := <-p.out:
		t.Fatalf("unexpected message for %s: %s", p.ID, b)
	default:
	}
}

func dmDrain(p *Peer) {
	for {
		select {
		case <-p.out:
		default:
			return
		}
	}
}

func dmSetPolicy(t *testing.T, h *Hub, r *Room, policy string, peers ...*Peer) {
	t.Helper()
	if err := h.setState(r, Msg{Value: json.RawMessage(`{"dmPolicy":"` + policy + `"}`)}); err != nil {
		t.Fatal(err)
	}
	for _, p := range peers {
		if m := dmRead(t, p); m.Type != "state" {
			t.Fatalf("expected state broadcast, got %s", m.Type)
		}
	}
}

// dmWelcome reads until the welcome arrives: a guest welcome is preceded by
// the room-persistent broadcast from notifyPersistence.
func dmWelcome(t *testing.T, p *Peer) Msg {
	t.Helper()
	for {
		if m := dmRead(t, p); m.Type == "welcome" {
			return m
		}
	}
}

func TestDMDeliversOnlyToPairWithEcho(t *testing.T) {
	h, r, host, g1, g2 := dmFixture(t)
	ctx := context.Background()
	if r.Snapshot().DMPolicy != "all" {
		t.Fatal("default dm policy must be all")
	}
	if err := h.handle(ctx, host, Msg{Type: "dm", To: g1.ID, Text: "hola Ana"}); err != nil {
		t.Fatal(err)
	}
	got := dmRead(t, g1)
	if got.Type != "dm" || got.Comment == nil || got.Comment.Text != "hola Ana" ||
		got.PeerID != host.ID || got.To != g1.ID ||
		got.Comment.PeerID != host.ID || got.Comment.To != g1.ID ||
		got.Comment.Source != "dm" || got.Comment.Role != RoleHost || got.Comment.From != "Host" {
		t.Fatalf("bad delivery: %+v", got)
	}
	echo := dmRead(t, host)
	if echo.Type != "dm" || echo.Comment == nil || echo.Comment.ID != got.Comment.ID || echo.To != g1.ID {
		t.Fatalf("sender echo missing or wrong: %+v", echo)
	}
	dmQuiet(t, g2)

	// Default policy allows guest -> guest as well.
	if err := h.handle(ctx, g1, Msg{Type: "dm", To: g2.ID, Text: "hi Bob"}); err != nil {
		t.Fatal(err)
	}
	if m := dmRead(t, g2); m.Type != "dm" || m.Comment.Text != "hi Bob" || m.PeerID != g1.ID {
		t.Fatalf("guest-to-guest delivery: %+v", m)
	}
	if m := dmRead(t, g1); m.Type != "dm" || m.Comment.Text != "hi Bob" {
		t.Fatalf("guest echo: %+v", m)
	}
	dmQuiet(t, host)
	if n := len(r.dmHistory(g1.ID)); n != 2 {
		t.Fatalf("g1 history %d, want 2", n)
	}
	if n := len(r.dmHistory(host.ID)); n != 1 {
		t.Fatalf("host history %d, want 1", n)
	}
}

func TestDMValidationAndStickers(t *testing.T) {
	h, r, host, g1, g2 := dmFixture(t)
	ctx := context.Background()

	// Text is trimmed and capped at 500 runes, like the room chat.
	if err := h.handle(ctx, host, Msg{Type: "dm", To: g1.ID, Text: "  " + strings.Repeat("x", 600) + "  "}); err != nil {
		t.Fatal(err)
	}
	m := dmRead(t, g1)
	if m.Comment == nil || len([]rune(m.Comment.Text)) != 500 {
		t.Fatalf("text not capped: %+v", m.Comment)
	}
	dmRead(t, host) // echo

	// Empty text is silently dropped.
	if err := h.handle(ctx, host, Msg{Type: "dm", To: g1.ID, Text: "   "}); err != nil {
		t.Fatal(err)
	}
	dmQuiet(t, g1)
	dmQuiet(t, host)

	// Stickers use the same whitelist as the chat.
	if err := h.handle(ctx, g1, Msg{Type: "dm", To: host.ID, Kind: "sticker", Text: "hola"}); err != nil {
		t.Fatal(err)
	}
	if m := dmRead(t, host); m.Comment == nil || m.Comment.Sticker != "hola" || m.Comment.Text != "" {
		t.Fatalf("sticker dm: %+v", m.Comment)
	}
	dmRead(t, g1) // echo
	if err := h.handle(ctx, g1, Msg{Type: "dm", To: host.ID, Kind: "sticker", Text: "<script>"}); err == nil {
		t.Fatal("unknown sticker accepted")
	}

	// Bad targets.
	for _, to := range []string{"", host.ID, "nobody"} {
		if err := h.handle(ctx, host, Msg{Type: "dm", To: to, Text: "x"}); err == nil {
			t.Fatalf("dm to %q accepted", to)
		}
	}
	dmQuiet(t, g1)
	dmQuiet(t, g2)
	// host->g1 and g1->host share one conversation: the capped text + sticker.
	if n := len(r.dmHistory(g1.ID)); n != 2 {
		t.Fatalf("invalid dms leaked into history: %d", n)
	}
}

func TestDMPolicyHostOnly(t *testing.T) {
	h, r, host, g1, g2 := dmFixture(t)
	ctx := context.Background()
	dmSetPolicy(t, h, r, "host-only", host, g1, g2)

	err := h.handle(ctx, g1, Msg{Type: "dm", To: g2.ID, Text: "x"})
	if err == nil || CodeOf(err) != "dm_host_only" {
		t.Fatalf("guest-to-guest under host-only: %v", err)
	}
	dmQuiet(t, g2)

	// Guest -> host and host -> guest stay allowed.
	if err := h.handle(ctx, g1, Msg{Type: "dm", To: host.ID, Text: "question"}); err != nil {
		t.Fatal(err)
	}
	if m := dmRead(t, host); m.Type != "dm" || m.Comment.Text != "question" {
		t.Fatalf("guest-to-host: %+v", m)
	}
	dmRead(t, g1) // echo
	if err := h.handle(ctx, host, Msg{Type: "dm", To: g2.ID, Text: "answer"}); err != nil {
		t.Fatal(err)
	}
	if m := dmRead(t, g2); m.Type != "dm" || m.Comment.Text != "answer" {
		t.Fatalf("host-to-guest: %+v", m)
	}
	dmRead(t, host) // echo

	// Unknown policies are rejected without touching the state.
	if err := h.setState(r, Msg{Value: json.RawMessage(`{"dmPolicy":"bogus"}`)}); err == nil {
		t.Fatal("bogus policy accepted")
	}
	if r.Snapshot().DMPolicy != "host-only" {
		t.Fatal("failed patch changed the policy")
	}
}

func TestDMPolicyOff(t *testing.T) {
	h, r, host, g1, g2 := dmFixture(t)
	ctx := context.Background()
	dmSetPolicy(t, h, r, "off", host, g1, g2)
	for _, pair := range [][2]*Peer{{host, g1}, {g1, host}, {g1, g2}} {
		err := h.handle(ctx, pair[0], Msg{Type: "dm", To: pair[1].ID, Text: "x"})
		if err == nil || CodeOf(err) != "dm_disabled" {
			t.Fatalf("dm under off policy: %v", err)
		}
	}
	dmQuiet(t, host)
	dmQuiet(t, g1)
	dmQuiet(t, g2)
	if len(r.dmHistory(host.ID)) != 0 || len(r.dmHistory(g1.ID)) != 0 {
		t.Fatal("rejected dms stored")
	}
}

func TestDMPolicyEmptyMeansAll(t *testing.T) {
	h, r, host, g1, g2 := dmFixture(t)
	// Rooms saved before dm existed carry no policy.
	if err := r.Update(func(st *State) error { st.DMPolicy = ""; return nil }); err != nil {
		t.Fatal(err)
	}
	dmDrain(host)
	dmDrain(g1)
	dmDrain(g2)
	if err := h.handle(context.Background(), g1, Msg{Type: "dm", To: g2.ID, Text: "x"}); err != nil {
		t.Fatalf("empty policy must mean all: %v", err)
	}
}

func TestDMViewerAndLobbyRejected(t *testing.T) {
	h, r, host, g1, _ := dmFixture(t)
	ctx := context.Background()
	viewer := newPeer(nil, nil)
	viewer.ID, viewer.Role, viewer.room = "obs", RoleViewer, r
	r.peers[viewer.ID] = viewer
	knocker := newPeer(nil, nil)
	knocker.ID, knocker.room = "knocker", r // Role "" = waiting in the lobby
	r.waiting[knocker.ID] = knocker

	if err := h.handle(ctx, viewer, Msg{Type: "dm", To: g1.ID, Text: "x"}); err == nil || CodeOf(err) != "dm_viewer" {
		t.Fatalf("viewer sent a dm: %v", err)
	}
	if err := h.handle(ctx, host, Msg{Type: "dm", To: viewer.ID, Text: "x"}); err == nil || CodeOf(err) != "dm_target_unavailable" {
		t.Fatalf("dm to a viewer accepted: %v", err)
	}
	if err := h.handle(ctx, knocker, Msg{Type: "dm", To: host.ID, Text: "x"}); err == nil {
		t.Fatal("lobby peer sent a dm")
	}
	if err := h.handle(ctx, host, Msg{Type: "dm", To: knocker.ID, Text: "x"}); err == nil || CodeOf(err) != "dm_target_unavailable" {
		t.Fatalf("dm to a waiting peer accepted: %v", err)
	}
	dmQuiet(t, g1)
	dmQuiet(t, viewer)

	// Welcome catch-up: the guest gets their dms, the viewer never does.
	if err := h.handle(ctx, host, Msg{Type: "dm", To: g1.ID, Text: "secret"}); err != nil {
		t.Fatal(err)
	}
	dmRead(t, g1)
	dmRead(t, host)
	h.welcome(viewer)
	if w := dmWelcome(t, viewer); len(w.Dms) != 0 {
		t.Fatalf("viewer welcome carries dms: %+v", w.Dms)
	}
	h.welcome(g1)
	w := dmWelcome(t, g1)
	if len(w.Dms) != 1 || w.Dms[0].Text != "secret" || w.Dms[0].To != g1.ID {
		t.Fatalf("guest welcome missing dm history: %+v", w.Dms)
	}
}

func TestDMHistoryFollowsRejoinPass(t *testing.T) {
	h, r, host, g1, _ := dmFixture(t)
	ctx := context.Background()
	pass := r.issuePass(g1)
	if err := h.handle(ctx, host, Msg{Type: "dm", To: g1.ID, Text: "before"}); err != nil {
		t.Fatal(err)
	}
	if err := h.handle(ctx, g1, Msg{Type: "dm", To: host.ID, Text: "reply"}); err != nil {
		t.Fatal(err)
	}
	dmDrain(host)
	dmDrain(g1)

	// Reconnect: a fresh peer id with the same rejoin pass (no h.leave here:
	// the fixture peers have no socket to close).
	delete(r.peers, g1.ID)
	g1b := newPeer(nil, nil)
	g1b.ID, g1b.Role, g1b.room, g1b.pass = "guest-one-back", RoleGuest, r, pass
	r.peers[g1b.ID] = g1b
	h.welcome(g1b)
	w := dmWelcome(t, g1b)
	if len(w.Dms) != 2 {
		t.Fatalf("rejoin welcome lost dm history: %+v", w.Dms)
	}
	byText := map[string]Comment{}
	for _, c := range w.Dms {
		byText[c.Text] = c
		if c.PeerID != host.ID && c.PeerID != g1b.ID {
			t.Fatalf("old peer id leaked into history: %+v", c)
		}
		if c.To != host.ID && c.To != g1b.ID {
			t.Fatalf("old recipient id leaked into history: %+v", c)
		}
	}
	if c := byText["before"]; c.PeerID != host.ID || c.To != g1b.ID {
		t.Fatalf("incoming dm not rebound: %+v", c)
	}
	if c := byText["reply"]; c.PeerID != g1b.ID || c.To != host.ID {
		t.Fatalf("outgoing dm not rebound: %+v", c)
	}
	if n := len(r.dmHistory(host.ID)); n != 2 {
		t.Fatalf("host history after rejoin: %d", n)
	}
	// New messages land in the same conversation.
	if err := h.handle(ctx, host, Msg{Type: "dm", To: g1b.ID, Text: "after"}); err != nil {
		t.Fatal(err)
	}
	if n := len(r.dmHistory(g1b.ID)); n != 3 {
		t.Fatalf("conversation split after rejoin: %d", n)
	}
}

func TestChatDeleteModeration(t *testing.T) {
	h, r, host, g1, g2 := dmFixture(t)
	ctx := context.Background()

	// Public chat: deletion is broadcast to everyone.
	if err := h.handle(ctx, g1, Msg{Type: "chat", Text: "bad word"}); err != nil {
		t.Fatal(err)
	}
	var id string
	for _, p := range []*Peer{host, g1, g2} {
		m := dmRead(t, p)
		if m.Type != "chat" || m.Comment == nil {
			t.Fatalf("expected chat broadcast, got %s", m.Type)
		}
		id = m.Comment.ID
	}
	// Feature it on the program output, then delete it.
	if err := r.Update(func(st *State) error { st.Comment = &Comment{ID: id, Text: "bad word"}; return nil }); err != nil {
		t.Fatal(err)
	}
	dmDrain(host)
	dmDrain(g1)
	dmDrain(g2)
	if err := h.handle(ctx, g1, Msg{Type: "chat-delete", Target: id}); err == nil {
		t.Fatal("guest deleted a message")
	}
	if err := h.handle(ctx, host, Msg{Type: "chat-delete", Target: id}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []*Peer{host, g1, g2} {
		if m := dmRead(t, p); m.Type != "chat-deleted" || m.ID != id {
			t.Fatalf("chat-deleted not broadcast: %+v", m)
		}
		dmRead(t, p) // state broadcast clearing the featured comment
	}
	if len(r.history()) != 0 {
		t.Fatal("deleted chat message kept in history")
	}
	if r.Snapshot().Comment != nil {
		t.Fatal("deleted message stayed on the program output")
	}

	// Direct message: only the pair is notified.
	if err := h.handle(ctx, host, Msg{Type: "dm", To: g1.ID, Text: "dm secret"}); err != nil {
		t.Fatal(err)
	}
	mid := dmRead(t, g1).Comment.ID
	dmRead(t, host) // echo
	dmQuiet(t, g2)
	if err := h.handle(ctx, host, Msg{Type: "chat-delete", Target: mid}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []*Peer{host, g1} {
		if m := dmRead(t, p); m.Type != "chat-deleted" || m.ID != mid {
			t.Fatalf("dm pair not notified: %+v", m)
		}
	}
	dmQuiet(t, g2)
	if len(r.dmHistory(g1.ID)) != 0 || len(r.dmHistory(host.ID)) != 0 {
		t.Fatal("deleted dm kept in history")
	}
	if err := h.handle(ctx, host, Msg{Type: "chat-delete", Target: "nope"}); err == nil {
		t.Fatal("unknown comment id accepted")
	}
}

func TestDMHistoryCap200(t *testing.T) {
	h, r, host, g1, _ := dmFixture(t)
	ctx := context.Background()
	for i := 0; i < 250; i++ {
		if err := h.handle(ctx, host, Msg{Type: "dm", To: g1.ID, Text: fmt.Sprintf("m%03d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	hist := r.dmHistory(g1.ID)
	if len(hist) != 200 {
		t.Fatalf("history %d, want 200", len(hist))
	}
	if hist[0].Text != "m050" || hist[199].Text != "m249" {
		t.Fatalf("cap dropped the wrong end: %s .. %s", hist[0].Text, hist[199].Text)
	}
}

func TestDMCheckedRecipientAtomic(t *testing.T) {
	_, r, host, g1, _ := dmFixture(t)
	if err := r.addDMChecked(Comment{ID: "x1", PeerID: host.ID, To: "ghost", Text: "hi"}); err == nil {
		t.Fatal("expected peer not found")
	}
	waiting := newPeer(nil, nil)
	waiting.ID, waiting.Role, waiting.room = "wait", RoleGuest, r
	r.waiting[waiting.ID] = waiting
	if err := r.addDMChecked(Comment{ID: "x2", PeerID: host.ID, To: waiting.ID, Text: "hi"}); err == nil || !strings.Contains(err.Error(), "cannot receive") {
		t.Fatalf("expected dm_target_unavailable, got %v", err)
	}
	if err := r.addDMChecked(Comment{ID: "x3", PeerID: host.ID, To: g1.ID, Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	if got := r.dmHistory(g1.ID); len(got) != 1 || got[0].ID != "x3" {
		t.Fatalf("history: %+v", got)
	}
}

func TestRebindKeepsMessagesArrivedBeforeRebind(t *testing.T) {
	_, r, host, g1, _ := dmFixture(t)
	for i := 0; i < 200; i++ {
		r.addDM(Comment{ID: fmt.Sprintf("old%d", i), PeerID: host.ID, To: "old-g1", Text: "o", TS: int64(100 + i)})
	}
	// Arrives on the fresh id before the reconnect rebinds the history.
	r.addDM(Comment{ID: "new", PeerID: host.ID, To: g1.ID, Text: "n", TS: 900})
	r.mu.Lock()
	r.rebindDMLocked("old-g1", g1.ID)
	r.mu.Unlock()
	hist := r.dmHistory(g1.ID)
	if len(hist) != 200 {
		t.Fatalf("len %d, want 200", len(hist))
	}
	if hist[len(hist)-1].ID != "new" {
		t.Fatalf("new message dropped by cap: %+v", hist[len(hist)-1])
	}
	if hist[0].ID != "old1" {
		t.Fatalf("oldest should be dropped first, got %s", hist[0].ID)
	}
}

func TestChatDeleteKeepsNewerFeaturedComment(t *testing.T) {
	h, r, host, g1, g2 := dmFixture(t)
	ctx := context.Background()
	if err := h.handle(ctx, g1, Msg{Type: "chat", Text: "first"}); err != nil {
		t.Fatal(err)
	}
	var first string
	for _, p := range []*Peer{host, g1, g2} {
		first = dmRead(t, p).Comment.ID
	}
	if err := h.handle(ctx, g1, Msg{Type: "chat", Text: "second"}); err != nil {
		t.Fatal(err)
	}
	var second string
	for _, p := range []*Peer{host, g1, g2} {
		second = dmRead(t, p).Comment.ID
	}
	dmDrain(host)
	dmDrain(g1)
	dmDrain(g2)
	if err := r.Update(func(st *State) error { st.Comment = &Comment{ID: second, Text: "second"}; return nil }); err != nil {
		t.Fatal(err)
	}
	dmDrain(host)
	dmDrain(g1)
	dmDrain(g2)
	if err := h.handle(ctx, host, Msg{Type: "chat-delete", Target: first}); err != nil {
		t.Fatal(err)
	}
	if st := r.Snapshot(); st.Comment == nil || st.Comment.ID != second {
		t.Fatalf("newer featured comment cleared: %+v", st.Comment)
	}
}
