package postgres

import (
	"encoding/json"
	"testing"
)

func TestCrmcallsTransferred(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"Передан лид", true},
		{" передан лид ", true},
		{"Заказ/лид", true},
		{"заказ/лид", true},
		{"Нет ответа", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := crmcallsTransferred(tc.in); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.in, got, tc.want)
		}
	}
}

func TestMergeCallFormImya(t *testing.T) {
	if mergeCallFormImya("", "Иван") != `{"imya":"Иван"}` {
		t.Fatalf("empty got %q", mergeCallFormImya("", "Иван"))
	}
	if mergeCallFormImya(`{"imya":"Пётр"}`, "Иван") != `{"imya":"Иван"}` {
		t.Fatalf("overwrite got %q", mergeCallFormImya(`{"imya":"Пётр"}`, "Иван"))
	}
	raw := mergeCallFormImya(`{"summa_dolga":"100"}`, "Иван")
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if m["imya"] != "Иван" || m["summa_dolga"] != "100" {
		t.Fatalf("%+v", m)
	}
}
