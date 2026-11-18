package postgres

import (
	"strings"
	"testing"
)

func TestDSNPoolSize(t *testing.T) {
	dsn := DSN("127.0.0.1", "5432", "u", "p", "sdmp", 32)
	if !strings.Contains(dsn, "pool_max_conns=32") {
		t.Fatalf("max: %s", dsn)
	}
	if !strings.Contains(dsn, "pool_min_conns=4") {
		t.Fatalf("min: %s", dsn)
	}
}

func TestDSNDefaultMaxConns(t *testing.T) {
	dsn := DSN("127.0.0.1", "5432", "u", "p", "sdmp", 0)
	if !strings.Contains(dsn, "pool_max_conns=32") {
		t.Fatalf("default max: %s", dsn)
	}
}
