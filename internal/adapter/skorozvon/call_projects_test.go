package skorozvon

import (
	"testing"
)

func TestDecodeCallProjectsWrapped(t *testing.T) {
	raw := []byte(`{
		"data": [
			{"id":50000167215,"title":"ВР КАБИНЕТ #1 (НОВЫЙ 2)","type":"call"},
			{"id":0,"title":"skip"},
			{"title":"no id"}
		],
		"pagination": {"total":2,"length":2,"page":1,"total_pages":1}
	}`)
	got, err := decodeCallProjects(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 50000167215 || got[0].Title != "ВР КАБИНЕТ #1 (НОВЫЙ 2)" {
		t.Fatalf("%+v", got)
	}
}

func TestDecodeCallProjectsArray(t *testing.T) {
	raw := []byte(`[{"id":9,"name":"Проект"}]`)
	got, err := decodeCallProjects(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 9 || got[0].Title != "Проект" {
		t.Fatalf("%+v", got)
	}
}
