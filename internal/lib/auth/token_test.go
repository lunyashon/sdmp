package auth

import (
	"testing"
	"time"
)

func TestIssueAndValid(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	tok, err := Issue("secret", now)
	if err != nil || tok == "" {
		t.Fatal(err)
	}
	if !Valid("secret", tok, now.Add(time.Hour)) {
		t.Fatal("fresh token must pass")
	}
	if Valid("other", tok, now) {
		t.Fatal("wrong secret")
	}
	if Valid("secret", tok, now.Add(tokenTTL+time.Minute)) {
		t.Fatal("expired")
	}
	if Valid("secret", "nope", now) || Valid("", tok, now) {
		t.Fatal("empty or garbage")
	}
}

func TestCheckPassword(t *testing.T) {
	if !CheckPassword("admin", "pw", "admin", "pw") {
		t.Fatal("match")
	}
	if CheckPassword("admin", "no", "admin", "pw") || CheckPassword("a", "pw", "admin", "pw") {
		t.Fatal("mismatch")
	}
	if CheckPassword("admin", "pw", "", "pw") {
		t.Fatal("unconfigured login")
	}
}
