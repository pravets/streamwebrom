package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/webr0m/streamwebrom/internal/config"
)

const testCreateToken = "defensor-del-estudio-2026"

// protectedEnv starts the fixture with -create-token set.
func protectedEnv(t *testing.T) *env {
	t.Helper()
	return newEnvWith(t, func(c *config.Config) { c.CreateToken = testCreateToken })
}

func postRoom(t *testing.T, e *env, headers map[string]string, body string) (int, map[string]any, http.Header) {
	t.Helper()
	req, err := http.NewRequest("POST", e.ts.URL+"/api/rooms", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out, res.Header
}

func rawConfig(t *testing.T, e *env) (string, map[string]any) {
	t.Helper()
	res, err := http.Get(e.ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return string(body), out
}

// An empty secret keeps the historical behaviour: the LAN, the desktop app and
// the Windows launcher create rooms without any credential.
func TestCreateRoomWithoutSecretStaysOpen(t *testing.T) {
	e := newEnv(t)
	status, _, _ := postRoom(t, e, nil, `{"room":"sin-secreto"}`)
	if status != http.StatusCreated {
		t.Fatalf("open creation must answer 201, got %d", status)
	}
	_, cfg := rawConfig(t, e)
	if cfg["createProtected"] != false {
		t.Fatalf("createProtected must be false without a secret, got %v", cfg["createProtected"])
	}
}

func TestCreateRoomRequiresSecret(t *testing.T) {
	e := protectedEnv(t)
	cases := []struct {
		name    string
		headers map[string]string
		body    string
		status  int
	}{
		{"no credential", nil, `{"room":"sin-credential"}`, http.StatusUnauthorized},
		{"wrong bearer", map[string]string{"Authorization": "Bearer no-es-el-secreto"}, `{"room":"mal-token"}`, http.StatusUnauthorized},
		{"short bearer", map[string]string{"Authorization": "Bearer corto"}, `{"room":"token-corto"}`, http.StatusUnauthorized},
		{"other scheme", map[string]string{"Authorization": "Basic " + testCreateToken}, `{"room":"otro-esquema"}`, http.StatusUnauthorized},
		{"empty bearer", map[string]string{"Authorization": "Bearer "}, `{"room":"bearer-vacio"}`, http.StatusUnauthorized},
		{"wrong X-Create-Token", map[string]string{"X-Create-Token": "no-es-el-secreto"}, `{"room":"x-mal"}`, http.StatusUnauthorized},
		{"both headers", map[string]string{"Authorization": "Bearer " + testCreateToken, "X-Create-Token": testCreateToken}, `{"room":"dos-cabeceras"}`, http.StatusUnauthorized},
		{"correct bearer", map[string]string{"Authorization": "Bearer " + testCreateToken}, `{"room":"bearer-ok"}`, http.StatusCreated},
		{"correct X-Create-Token", map[string]string{"X-Create-Token": testCreateToken}, `{"room":"x-ok"}`, http.StatusCreated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body, header := postRoom(t, e, tc.headers, tc.body)
			if status != tc.status {
				t.Fatalf("got %d, want %d (%v)", status, tc.status, body)
			}
			if status == http.StatusCreated {
				return
			}
			// Stable code for scripts and clients; the text is for humans.
			if body["code"] != "room_create_forbidden" {
				t.Fatalf("missing stable code: %v", body)
			}
			if msg, _ := body["error"].(string); msg == "" {
				t.Fatalf("missing error text: %v", body)
			}
			if header.Get("WWW-Authenticate") == "" {
				t.Fatal("a rejected creation must advertise the Bearer scheme")
			}
		})
	}
}

// The secret must never reach the browser bundle or the logs.
func TestConfigNeverLeaksTheSecret(t *testing.T) {
	e := protectedEnv(t)
	body, cfg := rawConfig(t, e)
	if cfg["createProtected"] != true {
		t.Fatalf("createProtected must be true, got %v", cfg["createProtected"])
	}
	if strings.Contains(body, testCreateToken) {
		t.Fatal("/api/config leaked the create token")
	}
}

// Wrong codes are supposed to cost the guess budget, never the room creation
// allowance: allowCreate would answer 429 long before the ten allowed guesses.
func TestWrongTokenDoesNotSpendCreateAllowance(t *testing.T) {
	e := protectedEnv(t)
	for i := 0; i < 7; i++ {
		status, _, _ := postRoom(t, e, map[string]string{"Authorization": "Bearer intento-fallido"}, `{"room":"intento"}`)
		if status != http.StatusUnauthorized {
			t.Fatalf("guess %d: got %d, want 401", i, status)
		}
	}
	status, body, _ := postRoom(t, e, map[string]string{"Authorization": "Bearer " + testCreateToken}, `{"room":"tras-los-fallos"}`)
	if status != http.StatusCreated {
		t.Fatalf("a valid token must still create a room after failed guesses, got %d (%v)", status, body)
	}
}

func TestCreateTokenGuessingIsRateLimited(t *testing.T) {
	e := protectedEnv(t)
	var limited int
	for i := 0; i < 40; i++ {
		status, body, _ := postRoom(t, e, map[string]string{"Authorization": "Bearer adivinando-el-secreto"}, `{"room":"adivinar"}`)
		switch status {
		case http.StatusUnauthorized:
		case http.StatusTooManyRequests:
			if body["code"] != "room_create_rate_limited" {
				t.Fatalf("missing rate-limit code: %v", body)
			}
			limited++
		default:
			t.Fatalf("unexpected status %d (%v)", status, body)
		}
	}
	if limited == 0 {
		t.Fatal("guessing was never rate limited")
	}
	if e.hub.Stats()["rooms"].(int) != 0 {
		t.Fatal("failed attempts created a room")
	}
}
