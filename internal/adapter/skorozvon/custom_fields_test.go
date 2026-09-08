package skorozvon

import (
	"testing"

	"github.com/lunyashon/sdmp/internal/port"
)

func TestDecodeCustomFieldsArray(t *testing.T) {
	raw := []byte(`[
		{"id":50000011780,"name":"test info","field_type":"text","lead_type":"lead","options":[]},
		{"id":50000011777,"title":"Источник","type":"select","options":[{"id":50000004560,"label":"ВР"}]}
	]`)
	got, err := decodeCustomFields(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	if got[0].ID != 50000011780 || got[0].Name != "test info" || got[0].FieldType != "text" {
		t.Fatalf("text %+v", got[0])
	}
	if got[1].Name != "Источник" || got[1].FieldType != "select" || len(got[1].Options) != 1 || got[1].Options[0] != (port.SkorozvonCustomFieldOption{ID: 50000004560, Label: "ВР"}) {
		t.Fatalf("select %+v", got[1])
	}
}

func TestDecodeCustomFieldsWrapped(t *testing.T) {
	raw := []byte(`{"data":[{"id":1,"name":"Город","field_type":"text"}]}`)
	got, err := decodeCustomFields(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 || got[0].Name != "Город" {
		t.Fatalf("%+v", got)
	}
}
