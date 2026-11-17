package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/webr0m/streamwebrom/internal/config"
	"github.com/webr0m/streamwebrom/internal/studio"
)

const testCreateToken = "correct-horse-battery-staple-1"

func protectedEnv(t *testing.T) *env {
	t.Helper()
	return newEnvWith(t, func(c *config.Config, _ *studio.Options) { c.CreateToken = testCreateToken })
}

type createResult struct {
	status int
	body   map[string]any
	header http.Header
}

func postRoom(t *testing.T, e *env, body string, headers map[string]string) createResult {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, e.ts.URL+"/api/rooms", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		if value == "\x00" { // sentinel: set an empty header value
			req.Header[key] = []string{""}
			continue
		}
		req.Header.Set(key, value)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := createResult{status: res.StatusCode, header: res.Header, body: map[string]any{}}
	_ = json.Unmarshal(raw, &out.body)
	if strings.Contains(string(raw), testCreateToken) || strings.Contains(string(raw), "#key") {
		t.Fatalf("secret leaked into the response: %s", raw)
	}
	return out
}

func TestCreateTokenGate(t *testing.T) {
	open := newEnv(t)
	if got := postRoom(t, open, `{"room":"open-room"}`, nil); got.status != http.StatusCreated {
		t.Fatalf("open server must keep creating rooms, got %d %v", got.status, got.body)
	}

	protected := protectedEnv(t)
	cases := []struct {
		name    string
		headers map[string]string
		status  int
		code    string
	}{
		{"no credential", nil, http.StatusUnauthorized, "room_create_forbidden"},
		{"wrong token", map[string]string{"Authorization": "Bearer wrong-token-12345678"}, http.StatusUnauthorized, "room_create_forbidden"},
		{"bearer without scheme", map[string]string{"Authorization": testCreateToken}, http.StatusUnauthorized, "room_create_forbidden"},
		{"query key", map[string]string{"X-Test-Query": "?key=" + testCreateToken}, http.StatusUnauthorized, "room_create_forbidden"},
		{"admin token", map[string]string{"Authorization": "Bearer " + testCreateToken + "x"}, http.StatusUnauthorized, "room_create_forbidden"},
		{"bearer and header", map[string]string{"Authorization": "Bearer " + testCreateToken, "X-Create-Token": testCreateToken}, http.StatusUnauthorized, "room_create_forbidden"},
		{"bearer", map[string]string{"Authorization": "Bearer " + testCreateToken}, http.StatusCreated, ""},
		{"lowercase scheme", map[string]string{"Authorization": "bearer " + testCreateToken}, http.StatusCreated, ""},
		{"alternate header", map[string]string{"X-Create-Token": testCreateToken}, http.StatusCreated, ""},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The query case really appends ?key=; the dummy header only records
			// that intent without sending it twice through the header path.
			url := protected.ts.URL + "/api/rooms"
			if tc.name == "query key" {
				url += "?key=" + testCreateToken
			}
			req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(`{"room":"gate-`+strconv.Itoa(i)+`"}`))
			req.Header.Set("Content-Type", "application/json")
			for key, value := range tc.headers {
				if strings.HasPrefix(key, "X-Test") {
					continue
				}
				req.Header.Set(key, value)
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.status {
				b, _ := io.ReadAll(res.Body)
				t.Fatalf("case %d: got %d, want %d (%s)", i, res.StatusCode, tc.status, b)
			}
			if res.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("case %d: secret response is cacheable", i)
			}
			if tc.status == http.StatusUnauthorized {
				if res.Header.Get("WWW-Authenticate") == "" {
					t.Fatal("401 must advertise the bearer challenge")
				}
				var body map[string]any
				_ = json.NewDecoder(res.Body).Decode(&body)
				if body["code"] != tc.code {
					t.Fatalf("case %d: code %v, want %s", i, body["code"], tc.code)
				}
			}
		})
	}

	// Rejected requests never create a room, and a refused name is still free.
	if rooms := protected.hub.Stats()["rooms"].(int); rooms != 3 {
		t.Fatalf("expected only the three authorized rooms, got %d", rooms)
	}
	for _, name := range []string{"gate-0", "gate-1", "gate-2", "gate-3", "gate-4", "gate-5"} {
		if protected.hub.Room(name) != nil {
			t.Fatalf("rejected request created room %q", name)
		}
	}
}

func TestCreateTokenFailuresDoNotSpendTheCreationBudget(t *testing.T) {
	e := protectedEnv(t)
	for i := 0; i < 6; i++ { // allowCreate permits five creations per window
		if got := postRoom(t, e, `{"room":"wrong-guess"}`, map[string]string{"Authorization": "Bearer nope-nope-nope-nope"}); got.status != http.StatusUnauthorized {
			t.Fatalf("guess %d: got %d", i, got.status)
		}
	}
	if got := postRoom(t, e, `{"room":"after-guesses"}`, map[string]string{"Authorization": "Bearer " + testCreateToken}); got.status != http.StatusCreated {
		t.Fatalf("wrong guesses burnt the creation budget: %d %v", got.status, got.body)
	}
}

func TestCreateTokenFailureBudgetAndRecovery(t *testing.T) {
	e := protectedEnv(t)
	for i := 0; i < createAuthBurstForTest; i++ {
		got := postRoom(t, e, `{}`, map[string]string{"Authorization": "Bearer wrong-token-12345678"})
		if got.status != http.StatusUnauthorized {
			t.Fatalf("guess %d: got %d, want 401", i, got.status)
		}
	}
	if got := postRoom(t, e, `{}`, map[string]string{"Authorization": "Bearer wrong-token-12345678"}); got.status != http.StatusTooManyRequests || got.body["code"] != "room_create_rate_limited" {
		t.Fatalf("exhausted budget must answer 429/room_create_rate_limited, got %d %v", got.status, got.body)
	}
	// The valid owner is not queued behind the guesses.
	if got := postRoom(t, e, `{"room":"owner-room"}`, map[string]string{"Authorization": "Bearer " + testCreateToken}); got.status != http.StatusCreated {
		t.Fatalf("valid token blocked by the failure budget: %d %v", got.status, got.body)
	}
}

// createAuthBurstForTest mirrors the budget the server applies per address.
const createAuthBurstForTest = 10

func TestClientConfigReportsCreateProtection(t *testing.T) {
	for name, e := range map[string]*env{"open": newEnv(t), "protected": protectedEnv(t)} {
		res, err := http.Get(e.ts.URL + "/api/config")
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		var cfg map[string]any
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		flag, ok := cfg["createProtected"].(bool)
		if !ok || flag != (name == "protected") {
			t.Fatalf("%s: createProtected=%v", name, cfg["createProtected"])
		}
		if strings.Contains(string(raw), testCreateToken) {
			t.Fatal("/api/config exposed the secret")
		}
	}
}

func TestCreateTokenNeverReachesLogs(t *testing.T) {
	e := protectedEnv(t)
	postRoom(t, e, `{"room":"logged-room"}`, map[string]string{"Authorization": "Bearer " + testCreateToken})
	postRoom(t, e, `{}`, map[string]string{"Authorization": "Bearer wrong-token-12345678"})
	if strings.Contains(e.logs.String(), testCreateToken) {
		t.Fatal("the create secret was logged")
	}
}
