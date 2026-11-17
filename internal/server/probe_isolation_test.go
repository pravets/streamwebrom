package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/webr0m/streamwebrom/internal/config"
	"github.com/webr0m/streamwebrom/internal/studio"
)

func createProbe(t *testing.T, e *env) studio.Keys {
	t.Helper()
	res, err := http.Post(e.ts.URL+"/api/route-check", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("route-check: %d %s", res.StatusCode, b)
	}
	var keys studio.Keys
	if err := json.NewDecoder(res.Body).Decode(&keys); err != nil {
		t.Fatal(err)
	}
	return keys
}

func TestProbesHaveTheirOwnQuota(t *testing.T) {
	e := newEnvWith(t, func(_ *config.Config, o *studio.Options) { o.MaxRooms = 2 })
	e.createRoom(t, `{"room":"show-one"}`)
	e.createRoom(t, `{"room":"show-two"}`)

	// The show quota is full, yet a diagnostic room still fits: probes are not
	// charged to -max-rooms.
	probe := createProbe(t, e)
	if got := postRoom(t, e, `{"room":"show-three"}`, nil); got.status != http.StatusConflict {
		t.Fatalf("a third show must hit the room cap, got %d %v", got.status, got.body)
	}
	if e.hub.Room(probe.Room) == nil || !e.hub.Room(probe.Room).IsRouteProbe() {
		t.Fatal("probe room missing or not flagged")
	}
	// Stats keeps counting everything so operators still see the whole server.
	if rooms := e.hub.Stats()["rooms"].(int); rooms != 3 {
		t.Fatalf("stats must count probes too, got %d", rooms)
	}
}

func TestProbePoolIsBoundedAndDoesNotBlockShows(t *testing.T) {
	e := newEnvWith(t, func(_ *config.Config, o *studio.Options) { o.MaxRooms = 2 })
	for i := 0; i < 32; i++ {
		if _, err := e.hub.CreateRoomRouteProbe("", ""); err != nil {
			t.Fatalf("probe %d: %v", i, err)
		}
	}
	// A full diagnostic pool answers 503 instead of pretending to measure.
	res, err := http.Post(e.ts.URL+"/api/route-check", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(res.Body).Decode(&body)
	if res.StatusCode != http.StatusServiceUnavailable || body["code"] != "route_probe_busy" {
		t.Fatalf("full probe pool: got %d %v", res.StatusCode, body)
	}
	// ...while the show quota stays free for real broadcasts.
	if got := postRoom(t, e, `{"room":"show-after-probes"}`, nil); got.status != http.StatusCreated {
		t.Fatalf("probes consumed the show quota: %d %v", got.status, got.body)
	}
}

func TestProbeCannotStartAShow(t *testing.T) {
	e := newEnv(t)
	probe := createProbe(t, e)

	host := e.dial(t)
	host.send(map[string]any{"type": "join", "room": probe.Room, "role": "host", "key": probe.HostKey, "name": "check"})
	welcome := host.expect("welcome")
	if welcome.Keys != nil || welcome.Persistence != nil || welcome.RelayAccess != nil {
		t.Fatal("a probe handed out show keys or persistence powers")
	}
	host.send(map[string]any{"type": "golive"})
	if err := host.expect("error"); err.Code != "route_probe_forbidden" {
		t.Fatalf("golive on a probe: code %q", err.Code)
	}
	host.send(map[string]any{"type": "record", "action": "start"})
	if err := host.expect("error"); err.Code != "route_probe_forbidden" {
		t.Fatalf("record on a probe: code %q", err.Code)
	}

	// One participant only: the probe is the measurement, not a room.
	second := e.dial(t)
	second.send(map[string]any{"type": "join", "room": probe.Room, "role": "host", "key": probe.HostKey})
	if err := second.expect("error"); err.Code != "room_full" {
		t.Fatalf("second participant on a probe: code %q", err.Code)
	}

	// Guests are refused outright, even though a probe has no guest password.
	guest := e.dial(t)
	guest.send(map[string]any{"type": "join", "room": probe.Room, "role": "guest", "name": "guest"})
	if err := guest.expect("error"); err.Code != "route_probe_forbidden" {
		t.Fatalf("guest on a probe: code %q", err.Code)
	}
}

func TestProbeHTTPEndpointsAreClosed(t *testing.T) {
	e := newEnv(t)
	probe := createProbe(t, e)

	for _, tc := range []struct {
		name, path, body string
	}{
		{"whip", "/whip/" + probe.Room + "/cam", "v=0"},
		{"whep", "/whep/" + probe.Room + "/cam", "v=0"},
	} {
		req, _ := http.NewRequest(http.MethodPost, e.ts.URL+tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/sdp")
		req.Header.Set("Authorization", "Bearer "+probe.HostKey)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("%s on a probe: %d", tc.name, res.StatusCode)
		}
	}

	req, _ := http.NewRequest(http.MethodPost, e.ts.URL+"/api/rooms/"+probe.Room+"/persistence", strings.NewReader(`{"days":0}`))
	req.Header.Set("Authorization", "Bearer "+probe.HostKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(res.Body).Decode(&body)
	if res.StatusCode != http.StatusForbidden || body["code"] != "route_probe_forbidden" {
		t.Fatalf("persistence on a probe: %d %v", res.StatusCode, body)
	}
}

func TestProbeSourceMustNotBeAProbe(t *testing.T) {
	e := newEnv(t)
	probe := createProbe(t, e)
	if _, err := e.hub.CreateRoomRouteProbe(probe.Room, ""); err == nil {
		t.Fatal("a probe was accepted as the source of another probe")
	}
}

func TestProbeFlagDoesNotDependOnTheName(t *testing.T) {
	e := newEnvWith(t, func(_ *config.Config, o *studio.Options) { o.MaxRooms = 1 })
	// A real show may pick any valid id, including the check-* prefix.
	e.createRoom(t, `{"room":"check-normal"}`)
	if e.hub.Room("check-normal").IsRouteProbe() {
		t.Fatal("a normal room was classified as a probe")
	}
	// The room quota is full, the diagnostic quota is not.
	createProbe(t, e)
	if got := postRoom(t, e, `{"room":"second-show"}`, nil); got.status != http.StatusConflict {
		t.Fatalf("expected the show cap to stay full, got %d", got.status)
	}
}
