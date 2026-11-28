package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
	// Guests carry a rejoin pass and own it in dmPeerByPass, exactly as after
	// welcome/issuePass: a dm to a guest without a live pass is rejected.
	g1.ID, g1.Role, g1.room, g1.pass = "guest-one", RoleGuest, r, "pass-one"
	g2.ID, g2.Role, g2.room, g2.pass = "guest-two", RoleGuest, r, "pass-two"
	r.dmPeerByPass = map[string]string{"pass-one": g1.ID, "pass-two": g2.ID}
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

// addDMForTest stores a dm without recipient validation so tests can seed
// history for peers that are gone or only partially simulated.
func (r *Room) addDMForTest(c Comment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.appendDMLocked(c)
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
	// Deleting an id that is already gone is a no-op, not an error: a double
	// click or a client retry must not surface a failure to the moderator.
	if err := h.handle(ctx, host, Msg{Type: "chat-delete", Target: "nope"}); err != nil {
		t.Fatalf("idempotent delete returned an error: %v", err)
	}
	dmQuiet(t, g1)
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
	if err := r.addDMChecked(Comment{ID: "x1", PeerID: host.ID, To: "ghost", Text: "hi"}); err == nil || CodeOf(err) != "dm_target_unavailable" {
		t.Fatalf("expected dm_target_unavailable for a vanished peer, got %v", err)
	}
	waiting := newPeer(nil, nil)
	waiting.ID, waiting.Role, waiting.room = "wait", RoleGuest, r
	r.waiting[waiting.ID] = waiting
	if err := r.addDMChecked(Comment{ID: "x2", PeerID: host.ID, To: waiting.ID, Text: "hi"}); err == nil || CodeOf(err) != "dm_target_unavailable" {
		t.Fatalf("expected dm_target_unavailable, got %v", err)
	}
	if err := r.addDMChecked(Comment{ID: "x3", PeerID: host.ID, To: g1.ID, Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	if got := r.dmHistory(g1.ID); len(got) != 1 || got[0].ID != "x3" {
		t.Fatalf("history: %+v", got)
	}
}

// A guest whose session no longer owns its rejoin pass is not a valid dm
// target: after a reconnect the old peer lingers in r.peers (until pongWait)
// while the conversation already moved to the fresh id, so a dm here would be
// written where nobody will ever read it.
func TestDMRejectedForStalePassOwner(t *testing.T) {
	_, r, host, g1, _ := dmFixture(t)
	r.mu.Lock()
	g1.pass = "pass-g1"
	r.dmPeerByPass["pass-g1"] = "fresh-g1" // the conversation moved to a new peer id
	r.mu.Unlock()
	if err := r.addDMChecked(Comment{ID: "s1", PeerID: host.ID, To: g1.ID, Text: "lost"}); err == nil || CodeOf(err) != "dm_target_unavailable" {
		t.Fatalf("stale pass owner accepted: %v", err)
	}
	if got := r.dmHistory(g1.ID); len(got) != 0 {
		t.Fatalf("message stored for a stale peer: %+v", got)
	}
	r.mu.Lock()
	r.dmPeerByPass["pass-g1"] = g1.ID // the live session owns it again
	r.mu.Unlock()
	if err := r.addDMChecked(Comment{ID: "s2", PeerID: host.ID, To: g1.ID, Text: "ok"}); err != nil {
		t.Fatal(err)
	}
}

func TestRebindKeepsMessagesArrivedBeforeRebind(t *testing.T) {
	_, r, host, g1, _ := dmFixture(t)
	// The old session is still in r.peers, as it is for pongWait after a
	// reconnect; the pass already points at the fresh id.
	old := newPeer(nil, nil)
	old.ID, old.Role, old.room, old.pass = "old-g1", RoleGuest, r, "pass-g1"
	r.peers[old.ID] = old
	for i := 0; i < maxHistory; i++ {
		r.addDMForTest(Comment{ID: fmt.Sprintf("old%d", i), PeerID: host.ID, To: old.ID, Text: "o", TS: int64(100 + i)})
	}
	// Arrives on the fresh id before the reconnect rebinds the history.
	r.addDMForTest(Comment{ID: "new", PeerID: host.ID, To: g1.ID, Text: "n", TS: 900})
	r.mu.Lock()
	r.rebindDMLocked(old.ID, g1.ID)
	r.mu.Unlock()
	hist := r.dmHistory(g1.ID)
	if len(hist) != maxHistory {
		t.Fatalf("len %d, want %d", len(hist), maxHistory)
	}
	if hist[len(hist)-1].ID != "new" {
		t.Fatalf("new message dropped by cap: %+v", hist[len(hist)-1])
	}
	if hist[0].ID != "old1" {
		t.Fatalf("oldest should be dropped first, got %s", hist[0].ID)
	}
}

// welcome.dms is capped across all conversations, not only per conversation:
// three full conversations (3*200 = 600) must be trimmed to maxWelcomeDMs.
func TestDMHistoryTotalCap(t *testing.T) {
	_, r, host, g1, g2 := dmFixture(t)
	partners := []string{host.ID, g2.ID, "guest-three"}
	per := maxHistory
	for pi, to := range partners {
		for i := 0; i < per; i++ {
			r.addDMForTest(Comment{ID: fmt.Sprintf("m%1d-%03d", pi, i), PeerID: g1.ID, To: to, Text: "x", TS: int64(pi*per + i)})
		}
	}
	hist := r.dmHistory(g1.ID)
	if len(hist) != maxWelcomeDMs {
		t.Fatalf("catch-up %d, want %d", len(hist), maxWelcomeDMs)
	}
	if want := fmt.Sprintf("m%1d-%03d", len(partners)-1, per-1); hist[len(hist)-1].ID != want {
		t.Fatalf("newest message lost: %s, want %s", hist[len(hist)-1].ID, want)
	}
	if want := fmt.Sprintf("m%1d-%03d", 0, len(partners)*per-maxWelcomeDMs); hist[0].ID != want {
		t.Fatalf("oldest kept is %s, want %s", hist[0].ID, want)
	}
}

// Two sessions cannot share one rejoin pass: the second join takes over and
// the first is told, so the dm history keyed by the pass has a single owner.
func TestRejoinPassEvictsPreviousSession(t *testing.T) {
	_, r, host, g1, g2 := dmFixture(t)
	_ = host
	r.mu.Lock()
	g1.pass = "shared-pass"
	r.dmPeerByPass["shared-pass"] = g1.ID
	fresh := newPeer(nil, nil)
	fresh.ID, fresh.Role, fresh.room = "guest-fresh", RoleGuest, r
	evicted := r.evictPassLocked("shared-pass", fresh.ID)
	r.mu.Unlock()
	if len(evicted) != 1 || evicted[0] != g1 {
		t.Fatalf("evicted %v, want the previous owner of the pass", evicted)
	}
	// A different pass, or the keeper itself, evicts nobody.
	r.mu.Lock()
	defer r.mu.Unlock()
	if got := r.evictPassLocked("another-pass", fresh.ID); len(got) != 0 {
		t.Fatalf("unrelated pass evicted %v", got)
	}
	if got := r.evictPassLocked("shared-pass", g1.ID); len(got) != 0 {
		t.Fatalf("keeper evicted itself: %v", got)
	}
	_ = g2
}

// dms, public chat and moderation run from different goroutines in production;
// this keeps the locking honest under -race.
func TestDMAndChatConcurrent(t *testing.T) {
	h, r, host, g1, g2 := dmFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(3)
		go func(i int) {
			defer wg.Done()
			if err := h.handle(context.Background(), g1, Msg{Type: "dm", To: host.ID, Text: fmt.Sprintf("c%d", i)}); err != nil {
				t.Errorf("dm: %v", err)
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			if err := h.handle(context.Background(), g2, Msg{Type: "chat", Text: fmt.Sprintf("p%d", i)}); err != nil {
				t.Errorf("chat: %v", err)
			}
		}(i)
		go func() {
			defer wg.Done()
			if err := h.handle(context.Background(), host, Msg{Type: "chat-delete", Target: "gone"}); err != nil {
				t.Errorf("delete: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := r.dmHistory(g1.ID); len(got) != 20 {
		t.Fatalf("dm history %d, want 20", len(got))
	}
}

// A replaced session must not reclaim the pass from the session that took it
// over: otherwise its late welcome would rebind the dm history backwards.
func TestReplacedSessionCannotReclaimPass(t *testing.T) {
	_, r, _, g1, _ := dmFixture(t)
	r.mu.Lock()
	g1.replaced = true
	r.mu.Unlock()
	pass := r.issuePass(g1) // takes r.mu itself
	r.mu.RLock()
	owner := r.dmPeerByPass["pass-one"]
	r.mu.RUnlock()
	if pass != "" {
		t.Fatalf("replaced session got a pass: %q", pass)
	}
	if owner != "guest-one" {
		t.Fatalf("pass rebound away from the live session: %q", owner)
	}
}

// The same stale-session rule applies to the sender: a replaced guest may not
// post into the conversation that was just moved away from it.
func TestDMRejectedForStaleSender(t *testing.T) {
	_, r, host, g1, _ := dmFixture(t)
	r.mu.Lock()
	old := newPeer(nil, nil)
	old.ID, old.Role, old.room, old.pass = "guest-old", RoleGuest, r, "pass-one"
	r.dmPeerByPass["pass-one"] = g1.ID // the pass already belongs to the fresh id
	r.peers[old.ID] = old
	r.mu.Unlock()
	if err := r.addDMChecked(Comment{ID: "t1", PeerID: old.ID, Role: RoleGuest, To: host.ID, Text: "lost"}); err == nil || CodeOf(err) != "dm_session_stale" {
		t.Fatalf("stale sender accepted: %v", err)
	}
	if got := r.dmHistory(host.ID); len(got) != 0 {
		t.Fatalf("stale sender stored a message: %+v", got)
	}
}

func TestChatDeleteClearsFeaturedOutsideHistory(t *testing.T) {
	h, r, host, g1, _ := dmFixture(t)
	ctx := context.Background()
	// The comment scrolled out of the ring buffer but is still on screen.
	if err := r.Update(func(st *State) error { st.Comment = &Comment{ID: "gone-cid", Text: "old"}; return nil }); err != nil {
		t.Fatal(err)
	}
	dmDrain(host)
	dmDrain(g1)
	if err := h.handle(ctx, host, Msg{Type: "chat-delete", Target: "gone-cid"}); err != nil {
		t.Fatalf("delete outside history failed: %v", err)
	}
	if st := r.Snapshot(); st.Comment != nil {
		t.Fatalf("featured comment not cleared: %+v", st.Comment)
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
