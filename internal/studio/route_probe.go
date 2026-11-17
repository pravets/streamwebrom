package studio

import (
	"time"
)

// Route probes are diagnostic rooms. They have their own small quota, a hard
// lifetime and a reduced set of powers, so a busy diagnostic endpoint can never
// fill the room cap of the server, nor start a recording or a live stream.
const (
	maxRouteProbes     = 32
	routeProbeLifetime = 45 * time.Second
)

var errRouteProbeBusy = Userf("route_probe_busy", "todas las comprobaciones de red están ocupadas, prueba en unos segundos")

// CreateRouteProbe creates a diagnostic room for the public network check.
func (h *Hub) CreateRouteProbe() (Keys, error) { return h.CreateRoomRouteProbe("", "") }

// CreateRoomRouteProbe copies the route only after validating the room password.
// The ordinary diagnostic endpoint remains local and cannot obtain CF access.
func (h *Hub) CreateRoomRouteProbe(room, password string) (Keys, error) {
	cloud := false
	if room != "" {
		r := h.Room(room)
		if r == nil || r.IsRouteProbe() || !r.CheckGuest(password) {
			return Keys{}, Userf("route_auth_invalid", "Sala o contraseña inválida")
		}
		cloud = r.CloudflareEnabled()
	}
	keys, err := h.createRoom("check-"+NewRoomID(), "", "Prueba de conexión", false, true)
	if err != nil {
		return Keys{}, err
	}
	h.mu.Lock()
	r := h.probes[keys.Room]
	h.mu.Unlock()
	if r == nil {
		return Keys{}, errRouteProbeBusy
	}
	r.mu.Lock()
	r.probeCloud = cloud
	r.mu.Unlock()
	// The lifetime is unconditional: activity never extends a diagnostic room.
	time.AfterFunc(routeProbeLifetime, func() { h.ReleaseRouteProbe(keys.Room, keys.HostKey) })
	return keys, nil
}

// ReleaseRouteProbe revokes one diagnostic room. It is idempotent and only
// accepts that room's own host key, so a stale timer can never free a show.
func (h *Hub) ReleaseRouteProbe(id, key string) bool {
	h.mu.Lock()
	r := h.probes[id]
	if r == nil || !r.CheckHost(key) {
		h.mu.Unlock()
		return false
	}
	delete(h.probes, id)
	if h.rooms[id] == r {
		delete(h.rooms, id)
	}
	h.mu.Unlock()
	go r.close()
	return true
}

// routeProbeMessage keeps a probe connection to the media handshake only.
func routeProbeMessage(kind string) bool {
	switch kind {
	case "ping", "ice", "publish", "unpublish", "subscribe", "unsubscribe", "answer":
		return true
	}
	return false
}

var errRouteProbeForbidden = Userf("route_probe_forbidden", "esta comprobación de red no permite esa acción")
