package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/port"
)

func TestLeadListWhereSkSendJoinsDispatch(t *testing.T) {
	sql, args := leadListWhere(port.LeadListFilter{
		DateFrom: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		DateTo:   time.Date(2026, 8, 30, 23, 59, 59, 0, time.UTC),
		SkSend:   []string{"error"},
	})
	if !strings.Contains(sql, "dispatch") || !strings.Contains(sql, "channel_id") || !strings.Contains(sql, "ANY(") {
		t.Fatalf("sql %s", sql)
	}
	if len(args) != 3 {
		t.Fatalf("args %#v sql %s", args, sql)
	}
	if leadListSkSelect(port.LeadListFilter{}) != `NULL::boolean, NULL::integer, ''::text` {
		t.Fatalf("select without filter")
	}
	if !strings.Contains(leadListSkJoin(port.LeadListFilter{SkSend: []string{"ok"}}), "LATERAL") {
		t.Fatalf("join with filter")
	}
	if leadListSkJoin(port.LeadListFilter{}) != "" {
		t.Fatalf("join without filter")
	}
}
