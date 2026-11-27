// Package studio implements rooms, roles and the live production state
// (stage, layout, branding, banners, featured comments, recording and
// multistreaming) shared by the host studio, guests and OBS outputs.
package studio

import (
	"cmp"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/webr0m/streamwebrom/internal/restream"
	"github.com/webr0m/streamwebrom/internal/sfu"
)

// Role of a participant.
type Role string

// Roles. Host = full control, Guest = can publish when allowed, Viewer = OBS
// output / audience (receive only).
const (
	RoleHost   Role = "host"
	RoleGuest  Role = "guest"
	RoleViewer Role = "viewer"
)

// Layouts supported by the compositor (web/js/layout.js mirrors this list).
var Layouts = []string{"solo", "split", "grid", "pip", "sidebar", "spotlight", "screen", "news"}

// Banner is an on-screen message (StreamYard "banners" / news ticker).
type Banner struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Ticker bool   `json:"ticker"`
}

// Comment is a chat or audience comment that can be shown on screen.
type Comment struct {
	ID     string `json:"id"`
	From   string `json:"from"`
	Text   string `json:"text"`
	Source string `json:"source"` // chat | youtube | twitch | ...
	TS     int64  `json:"ts"`
	// Sticker is a built-in sticker id (web/js/stickers.js) instead of text.
	Sticker string `json:"sticker,omitempty"`
	PeerID  string `json:"peerId,omitempty"` // author, so clients can tell their own messages
	Role    Role   `json:"role,omitempty"`   // author's role ("host" gets a badge)
	To      string `json:"to,omitempty"`     // dm recipient peer id; empty for room chat
}

// Brand controls the look of the program output.
type Brand struct {
	Color      string `json:"color"`      // accent (lower thirds, banners)
	Background string `json:"background"` // css color or image URL
	Logo       string `json:"logo"`       // image URL (data: or https:)
	Overlay    string `json:"overlay"`    // full-frame transparent PNG URL
	Theme      string `json:"theme"`      // bubble | minimal | bold
	ShowNames  bool   `json:"showNames"`
}

// State is everything an output needs to render the program identically.
type State struct {
	Title     string            `json:"title"`
	Layout    string            `json:"layout"`
	Stage     []string          `json:"stage"`    // stream ids on stage, in order
	Featured  string            `json:"featured"` // stream id spotlighted
	Banner    *Banner           `json:"banner"`
	Banners   []Banner          `json:"banners"`
	Comment   *Comment          `json:"comment"`
	Brand     Brand             `json:"brand"`
	Lobby     bool              `json:"lobby"`
	Live      bool              `json:"live"`
	Recording []string          `json:"recording"`
	Restream  *restream.Status  `json:"restream,omitempty"`
	Scenes    map[string]Scene  `json:"scenes"`
	Outputs   map[string]Output `json:"outputs"`   // live scenes 2..8 (and custom), each with its own lineup
	Labels    map[string]string `json:"labels"`    // stream id -> display name / lower third
	Countdown int64             `json:"countdown"` // unix ms target, 0 = none

	// Program audio/video moderation (does not touch the guest's devices).
	Muted      map[string]bool    `json:"muted"`      // stream id -> muted in the program mix
	Volumes    map[string]float64 `json:"volumes"`    // stream id -> gain 0..2
	StageMuted bool               `json:"stageMuted"` // whole stage muted
	Hidden     map[string]bool    `json:"hidden"`     // stream id -> video hidden, audio kept
	Fit        map[string]string  `json:"fit"`        // stream id -> contain | cover for media sources
	Bandwidth  string             `json:"bandwidth"`  // eco | balanced | quality; applies to camera/screen senders
	Connection string             `json:"connection"` // local by default; cloudflare requires host/master authorization
	// DMPolicy gates direct messages: all (default), host-only (guests may
	// only write to a host) or off. Empty means all.
	DMPolicy string `json:"dmPolicy,omitempty"`
}

// Scene is a saved layout + stage combination (one-click switching).
type Scene struct {
	Layout string   `json:"layout"`
	Stage  []string `json:"stage"`
	Banner *Banner  `json:"banner"`
}

// Output is a live scene other than the program: its own lineup and layout,
// rendered by /view?scene=NAME (VDO.Ninja "add to scene N").
type Output struct {
	Stage    []string `json:"stage"`
	Layout   string   `json:"layout"`
	Featured string   `json:"featured"`
}

// Room is a production.
type Room struct {
	ID      string
	Created time.Time

	hostHash  []byte
	guestHash []byte
	viewKey   string
	pushKey   string
	inviteKey string // carried by the host's invite links: those guests skip the waiting room

	mu             sync.RWMutex
	peers          map[string]*Peer
	waiting        map[string]*Peer
	passes         map[string]bool // rejoin passes of admitted guests (reconnects skip the lobby)
	streams        map[string]*sfu.Stream
	publishing     map[string]bool
	state          State
	chat           []Comment
	dms            map[string][]Comment // conversation key (sorted peer ids) -> history
	dmPeerByPass   map[string]string    // rejoin pass -> last peer id, so dm history survives reconnects
	emptyAt        time.Time
	restream       *restream.Session
	starting       bool // a go-live is being set up (guards against double start)
	liveGeneration uint64
	persistent     bool
	persistDays    int
	expires        int64
	options        json.RawMessage
	participants   map[string]string
	savedSources   map[string]sfu.Info
	streamOwners   map[string]string
	onSave         func()
	closed         bool
	probe          bool // short-lived diagnostic room: own quota, reduced powers
	probeCloud     bool // short-lived check cloned only after room authentication
}

// IsRouteProbe reports whether this is a short-lived diagnostic room. Probes
// are never shows: they spend a separate quota and expose fewer powers.
func (r *Room) IsRouteProbe() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.probe
}

// Keys returned once to the creator.
type Keys struct {
	Room    string `json:"room"`
	HostKey string `json:"hostKey"`
	ViewKey string `json:"viewKey"`
	PushKey string `json:"pushKey"`
	// InviteKey goes in the invite links the studio hands out (#invite=…):
	// guests who come with it go straight to the backstage.
	InviteKey string `json:"inviteKey,omitempty"`
}

// NewRoomID makes an unguessable, URL-friendly id (50 bits).
func NewRoomID() string { return token(10) }

func token(n int) string {
	b := make([]byte, (n*5+7)/8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	s := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
	return s[:n]
}

func newRoom(id, guestPassword string, lobby bool, title string) (*Room, Keys, error) {
	hostKey := token(24)
	// High-entropy generated keys need constant-time verification, not an
	// expensive password KDF on every HTTP/WebSocket authentication attempt.
	hh := sha256.Sum256([]byte(hostKey))
	r := &Room{
		ID: id, Created: time.Now(), hostHash: hh[:],
		viewKey: token(16), pushKey: token(20), inviteKey: token(16),
		peers: map[string]*Peer{}, waiting: map[string]*Peer{}, passes: map[string]bool{}, streams: map[string]*sfu.Stream{},
		emptyAt: time.Now(), participants: map[string]string{}, savedSources: map[string]sfu.Info{}, streamOwners: map[string]string{},
		state: State{Title: title, Layout: "grid", Lobby: lobby, Connection: "local", Bandwidth: "balanced", DMPolicy: "all", Scenes: map[string]Scene{},
			Labels: map[string]string{},
			Brand:  Brand{Color: "#7c5cff", Background: "#0b0b12", Theme: "bubble", ShowNames: true}},
	}
	if guestPassword != "" {
		gh, err := bcrypt.GenerateFromPassword([]byte(guestPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, Keys{}, err
		}
		r.guestHash = gh
	}
	return r, Keys{Room: id, HostKey: hostKey, ViewKey: r.viewKey, PushKey: r.pushKey, InviteKey: r.inviteKey}, nil
}

// CheckHost verifies a host key.
func (r *Room) CheckHost(key string) bool {
	hash := sha256.Sum256([]byte(key))
	return key != "" && subtle.ConstantTimeCompare(r.hostHash, hash[:]) == 1
}

// CheckGuest verifies the guest password (always true when none is set).
func (r *Room) CheckGuest(pw string) bool {
	if r.guestHash == nil {
		return true
	}
	return bcrypt.CompareHashAndPassword(r.guestHash, []byte(pw)) == nil
}

// CheckView verifies a view key (constant time).
func (r *Room) CheckView(k string) bool {
	return subtle.ConstantTimeCompare([]byte(k), []byte(r.viewKey)) == 1
}

// CheckPush verifies a WHIP push key (constant time).
func (r *Room) CheckPush(k string) bool {
	return subtle.ConstantTimeCompare([]byte(k), []byte(r.pushKey)) == 1
}

// CheckInvite verifies the invite key of the studio's invite links.
func (r *Room) CheckInvite(k string) bool {
	return k != "" && subtle.ConstantTimeCompare([]byte(k), []byte(r.inviteKey)) == 1
}

// Stream returns a published stream.
func (r *Room) Stream(id string) *sfu.Stream {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.streams[id]
}

// Streams lists stream infos.
func (r *Room) Streams() []sfu.Info {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]sfu.Info, 0, len(r.streams))
	for _, s := range r.streams {
		out = append(out, s.Info())
	}
	return out
}

// AddStream registers a stream; an existing stream with the same id is replaced.
func (r *Room) AddStream(s *sfu.Stream) bool {
	return r.addStream(s, nil)
}

func (r *Room) addStream(s *sfu.Stream, permit func() bool) bool {
	r.mu.Lock()
	if r.closed || (permit != nil && !permit()) {
		r.mu.Unlock()
		s.Close()
		return false
	}
	old := r.streams[s.ID]
	r.streams[s.ID] = s
	r.mu.Unlock()
	if old != nil && old != s {
		old.Close()
	}
	s.OnClose(func() {
		r.mu.Lock()
		removed := r.streams[s.ID] == s
		if removed {
			delete(r.streams, s.ID)
		}
		r.mu.Unlock()
		if removed {
			r.broadcast(Msg{Type: "stream-removed", Stream: s.ID, Revision: s.Revision}, "")
		}
	})
	go func() {
		select {
		case <-s.Ready():
		case <-s.Done():
			return
		}
		announced := -1
		for {
			if r.Stream(s.ID) != s {
				return
			}
			inf := s.Info()
			if len(inf.Tracks) > announced {
				kind := "stream-updated"
				if announced < 0 {
					kind = "stream-added"
				}
				announced = len(inf.Tracks)
				r.broadcast(Msg{Type: kind, Info: &inf, Name: r.label(s)}, "")
			}
			select {
			case <-s.TracksChanged():
			case <-s.Done():
				return
			}
		}
	}()
	return true
}

func (r *Room) label(s *sfu.Stream) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if l := r.state.Labels[s.ID]; l != "" {
		return l
	}
	if p := r.peers[s.Owner]; p != nil {
		return p.Name
	}
	return s.ID
}

// Snapshot returns a copy of the state.
func (r *Room) Snapshot() State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, _ := cloneState(r.state)
	return s
}

// cloneState deep-copies a state (maps, slices and pointers included).
func cloneState(in State) (State, error) {
	s := in
	s.Stage = slices.Clone(in.Stage)
	s.Recording = slices.Clone(in.Recording)
	s.Banners = slices.Clone(in.Banners)
	s.Labels = maps.Clone(in.Labels)
	s.Muted = maps.Clone(in.Muted)
	s.Hidden = maps.Clone(in.Hidden)
	s.Fit = maps.Clone(in.Fit)
	s.Volumes = maps.Clone(in.Volumes)
	s.Scenes = maps.Clone(in.Scenes)
	for k, sc := range s.Scenes {
		sc.Stage = slices.Clone(sc.Stage)
		if sc.Banner != nil {
			b := *sc.Banner
			sc.Banner = &b
		}
		s.Scenes[k] = sc
	}
	s.Outputs = maps.Clone(in.Outputs)
	for k, o := range s.Outputs {
		o.Stage = slices.Clone(o.Stage)
		s.Outputs[k] = o
	}
	if in.Banner != nil {
		b := *in.Banner
		s.Banner = &b
	}
	if in.Comment != nil {
		c := *in.Comment
		s.Comment = &c
	}
	if in.Restream != nil {
		rs := *in.Restream
		rs.Targets = slices.Clone(rs.Targets)
		s.Restream = &rs
	}
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	if s.Scenes == nil {
		s.Scenes = map[string]Scene{}
	}
	return s, nil
}

// Update mutates the state under lock and broadcasts it. The change is
// all-or-nothing: fn works on a deep copy that replaces the state only when
// fn succeeds, so a patch rejected half-way (e.g. a valid title followed by
// an unknown layout) never leaves the room with a state nobody was told about.
func (r *Room) Update(fn func(*State) error) error {
	r.mu.Lock()
	next, err := cloneState(r.state)
	if err == nil {
		err = fn(&next)
	}
	if err != nil {
		r.mu.Unlock()
		return err
	}
	r.state = next
	// Removing a guest from every live scene also revokes screen access.
	removed := map[*Peer][]*sfu.Stream{}
	for _, p := range r.peers {
		p.mu.Lock()
		active := p.screenAllowed || p.screenRequested
		p.mu.Unlock()
		if active && !r.onStageLocked(p) {
			removed[p] = r.clearScreenLocked(p)
		}
	}
	r.mu.Unlock()
	for p, streams := range removed {
		r.finishScreenRemoval(p, streams, "revoked")
	}
	st := r.Snapshot()
	r.broadcast(Msg{Type: "state", State: &st}, "")
	if r.onSave != nil {
		r.onSave()
	}
	return nil
}

// reserveStream prevents concurrent negotiations from stealing the same ID.
func (r *Room) reserveStream(id, owner string, replaceAny bool, max int) (func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, errors.New("room closed")
	}
	if r.publishing[id] {
		return nil, errors.New("stream is being negotiated")
	}
	if old := r.streams[id]; old != nil && old.Owner != owner && !replaceAny {
		return nil, errors.New("stream id in use")
	}
	if r.streams[id] == nil && len(r.streams)+len(r.publishing) >= max {
		return nil, errors.New("room stream capacity reached")
	}
	if r.publishing == nil {
		r.publishing = map[string]bool{}
	}
	r.publishing[id] = true
	return func() { r.mu.Lock(); delete(r.publishing, id); r.mu.Unlock() }, nil
}

func (r *Room) broadcast(m Msg, except string) {
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for id, p := range r.peers {
		if id != except {
			p.sendRaw(b)
		}
	}
}

func (r *Room) toHosts(m Msg) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.peers {
		if p.role() == RoleHost {
			p.Send(m)
		}
	}
}

// PeerInfo is the public view of a participant.
type PeerInfo struct {
	Network         *NetworkInfo `json:"network,omitempty"`
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Role            Role         `json:"role"`
	Mic             bool         `json:"mic"`
	Cam             bool         `json:"cam"`
	Hand            bool         `json:"hand"`
	Stage           bool         `json:"stage"`
	ScreenAllowed   bool         `json:"screenAllowed"`
	ScreenRequested bool         `json:"screenRequested"`
}

// Peers returns public infos (viewers are hidden).
func (r *Room) Peers() []PeerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []PeerInfo{}
	for _, p := range r.peers {
		if p.Role != RoleViewer {
			out = append(out, p.Info(r.state.Stage))
		}
	}
	return out
}

func (r *Room) empty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.peers) == 0 && len(r.waiting) == 0 && len(r.streams) == 0
}

func (r *Room) close() {
	r.mu.Lock()
	r.closed = true
	r.liveGeneration++
	streams := make([]*sfu.Stream, 0, len(r.streams))
	for _, s := range r.streams {
		streams = append(streams, s)
	}
	rs := r.restream
	r.restream = nil
	peers := make([]*Peer, 0, len(r.peers)+len(r.waiting))
	for _, p := range r.peers {
		peers = append(peers, p)
	}
	for _, p := range r.waiting {
		peers = append(peers, p)
	}
	r.mu.Unlock()
	for _, p := range peers {
		p.close()
	}
	if rs != nil {
		rs.Stop()
	}
	for _, s := range streams {
		s.Close()
	}
}

func (r *Room) addChat(c Comment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chat = trimHistory(append(r.chat, c))
}

func (r *Room) history() []Comment {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Comment(nil), r.chat...)
}

// maxHistory caps the room chat and every dm conversation.
const maxHistory = 200

// maxWelcomeDMs caps the catch-up history a reconnecting peer receives across
// all of its conversations, so welcome does not grow with the room size.
const maxWelcomeDMs = 500

// trimHistory keeps the newest maxHistory comments.
func trimHistory(list []Comment) []Comment {
	if len(list) > maxHistory {
		return list[len(list)-maxHistory:]
	}
	return list
}

// addDMChecked validates the policy, the recipient and stores the dm under a
// single lock: neither a policy change nor a reconnect may land between the
// check and the append.
func (r *Room) addDMChecked(c Comment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch dmPolicyOf(r.state) {
	case "off":
		return Userf("dm_disabled", "Los mensajes directos están desactivados en esta sala")
	case "host-only":
		if c.Role != RoleHost {
			if host := r.peers[c.To]; host == nil || host.role() != RoleHost {
				return Userf("dm_host_only", "Los invitados solo pueden escribir al anfitrión")
			}
		}
	}
	t := r.peers[c.To]
	if t == nil {
		return Userf("dm_target_unavailable", "Ese participante no puede recibir mensajes directos")
	}
	tr := t.role()
	if tr != RoleHost && tr != RoleGuest {
		return Userf("dm_target_unavailable", "Ese participante no puede recibir mensajes directos")
	}
	// A guest address is only live while its session still owns the rejoin
	// pass: after a reconnect the old peer lingers in r.peers until pongWait,
	// but its conversation already moved to the fresh id, so a dm here would
	// be stored where nobody reads it.
	if tr == RoleGuest && (t.pass == "" || r.dmPeerByPass[t.pass] != t.ID) {
		return Userf("dm_target_unavailable", "Ese participante no puede recibir mensajes directos")
	}
	// The sender matters for the same reason: a replaced session keeps its
	// socket for a moment and could still post into the conversation that was
	// just rebound away from it.
	if c.Role == RoleGuest {
		s := r.peers[c.PeerID]
		if s == nil || s.pass == "" || r.dmPeerByPass[s.pass] != s.ID {
			return Userf("dm_session_stale", "Tu sesión ya no está activa en esta sala")
		}
	}
	r.appendDMLocked(c)
	return nil
}

// appendDMLocked appends one comment to its conversation and trims the oldest
// messages over the cap. Callers hold r.mu.
func (r *Room) appendDMLocked(c Comment) {
	if r.dms == nil {
		r.dms = map[string][]Comment{}
	}
	key := dmKey(c.PeerID, c.To)
	r.dms[key] = trimHistory(append(r.dms[key], c))
}

// dmHistory returns the catch-up history of every conversation this peer takes
// part in, oldest first, capped overall so welcome does not grow with the room.
func (r *Room) dmHistory(peerID string) []Comment {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Comment
	for key, msgs := range r.dms {
		if a, b, ok := dmKeyParts(key); ok && (a == peerID || b == peerID) {
			out = append(out, msgs...)
		}
	}
	slices.SortStableFunc(out, func(a, b Comment) int { return cmp.Compare(a.TS, b.TS) })
	if len(out) > maxWelcomeDMs {
		out = out[len(out)-maxWelcomeDMs:]
	}
	return out
}

// deleteComment removes one message by id from the room chat or a dm
// conversation. It reports where the message lived so the hub can notify
// exactly the affected peers: everyone for chat, the pair for a dm.
func (r *Room) deleteComment(id string) (inChat bool, pair [2]string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, c := range r.chat {
		if c.ID == id {
			r.chat = slices.Delete(r.chat, i, i+1)
			return true, [2]string{}, true
		}
	}
	for key, msgs := range r.dms {
		for i, c := range msgs {
			if c.ID == id {
				r.dms[key] = slices.Delete(msgs, i, i+1)
				if len(r.dms[key]) == 0 {
					delete(r.dms, key)
				}
				a, b, _ := dmKeyParts(key)
				return false, [2]string{a, b}, true
			}
		}
	}
	return false, [2]string{}, false
}

// rebindDMLocked moves a reconnecting guest's conversations from their old
// peer id to the new one, so dm history follows the rejoin pass the way stage
// slots do. Callers hold r.mu.
func (r *Room) rebindDMLocked(oldID, newID string) {
	for key, msgs := range r.dms {
		a, b, ok := dmKeyParts(key)
		if !ok || (a != oldID && b != oldID) {
			continue
		}
		delete(r.dms, key)
		if a == oldID {
			a = newID
		}
		if b == oldID {
			b = newID
		}
		for i := range msgs {
			if msgs[i].PeerID == oldID {
				msgs[i].PeerID = newID
			}
			if msgs[i].To == oldID {
				msgs[i].To = newID
			}
		}
		nk := dmKey(a, b)
		merged := append(r.dms[nk], msgs...)
		// Chronological merge: the cap must drop the oldest messages, never
		// one that arrived on the new id before the rebind.
		slices.SortStableFunc(merged, func(x, y Comment) int { return cmp.Compare(x.TS, y.TS) })
		r.dms[nk] = trimHistory(merged)
	}
}

var errFull = Userf("room_full", "room is full")

// maxPasses bounds the rejoin passes a room remembers.
const maxPasses = 1000

// issuePass returns the guest's rejoin pass, minting one on first welcome.
func (r *Room) issuePass(p *Peer) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	// A replaced session is on its way out: it must neither keep nor be given a
	// pass, otherwise its late welcome would rebind the pass (and the dm
	// history) away from the session that just took it over.
	if p.replaced {
		p.pass = ""
		return ""
	}
	if p.pass == "" || !r.passes[p.pass] {
		if len(r.passes) >= maxPasses && !r.persistent {
			r.passes = map[string]bool{}
			r.participants = map[string]string{}
			r.dmPeerByPass = map[string]string{}
		}
		p.pass = token(24)
		r.passes[p.pass] = true
	}
	r.participants[p.pass] = p.Name
	// A reconnect gets a fresh peer id: carry the dm conversations over.
	if old := r.dmPeerByPass[p.pass]; old != "" && old != p.ID {
		r.rebindDMLocked(old, p.ID)
	}
	if r.dmPeerByPass == nil {
		r.dmPeerByPass = map[string]string{}
	}
	r.dmPeerByPass[p.pass] = p.ID
	return p.pass
}

// evictPassLocked lists the live peers that still hold pass, excluding keep.
// The caller releases r.mu before telling them and closing their sockets.
func (r *Room) evictPassLocked(pass, keep string) []*Peer {
	if pass == "" {
		return nil
	}
	var out []*Peer
	for id, old := range r.peers {
		if id != keep && old.pass == pass {
			out = append(out, old)
		}
	}
	return out
}

// revokePass makes a kicked guest go through the lobby again.
func (r *Room) revokePass(p *Peer) {
	r.mu.Lock()
	delete(r.participants, p.pass)
	delete(r.passes, p.pass)
	delete(r.dmPeerByPass, p.pass)
	p.pass = ""
	r.mu.Unlock()
}

// passStreamID is a guest's stable stream id derived from their rejoin pass
// (hashed, since stream ids are visible to everyone in the room).
func passStreamID(pass, kind string) string {
	sum := sha256.Sum256([]byte("stream:" + pass))
	return "g" + hex.EncodeToString(sum[:6]) + "-" + kind
}
