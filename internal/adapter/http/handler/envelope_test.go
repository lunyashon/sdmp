package handler_test

import (
	"encoding/json"
	"testing"

	"github.com/lunyashon/sdmp/internal/adapter/http/response"
)

func decodeData[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var env response.Envelope[T]
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode envelope: %v body %s", err, raw)
	}
	if !env.Success {
		t.Fatalf("success=false body %s", raw)
	}
	return env.Data
}
