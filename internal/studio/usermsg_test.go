package studio

import (
	"errors"
	"testing"
)

func TestUserErrorsExposeStableCodes(t *testing.T) {
	if got := CodeOf(Userf("room_expired", "La sala caducó")); got != "room_expired" {
		t.Fatalf("CodeOf(user error) = %q", got)
	}
	if got := CodeOf(errors.New("boom")); got != "" {
		t.Fatalf("an internal error reported a code: %q", got)
	}

	msg := ErrorMsg(Userf("room_full", "La sala está llena"), "req-1", "join")
	if msg.Type != "error" || msg.Code != "room_full" || msg.Error != "La sala está llena" || msg.ReqID != "req-1" || msg.Action != "join" {
		t.Fatalf("unexpected error message: %+v", msg)
	}

	internal := ErrorMsg(errors.New("boom"), "", "")
	if internal.Code != "" || internal.Error != "boom" {
		t.Fatalf("internal failure leaked a code: %+v", internal)
	}
}
