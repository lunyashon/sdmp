package usecase

import (
	"context"
	"testing"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type stubGeoMaps struct {
	fields map[[2]int64]entity.SkorozvonMap
}

func (s stubGeoMaps) ReplaceAll(context.Context, []entity.SkorozvonMap) error { return nil }
func (s stubGeoMaps) Get(_ context.Context, fieldID, optionID int64) (entity.SkorozvonMap, bool, error) {
	row, ok := s.fields[[2]int64{fieldID, optionID}]
	return row, ok, nil
}

func TestEnrichSnapGeoFromCustomFields(t *testing.T) {
	maps := stubGeoMaps{fields: map[[2]int64]entity.SkorozvonMap{
		{50000009406, 0}:           {FieldID: 50000009406, Name: "Регион клиента", FieldType: "multiselect"},
		{50000009406, 50000015671}: {FieldID: 50000009406, OptionID: 50000015671, Name: "Амурская область"},
		{50000011777, 0}:           {FieldID: 50000011777, Name: "Город клиента", FieldType: "multiselect"},
		{50000011777, 50000004560}: {FieldID: 50000011777, OptionID: 50000004560, Name: "Благовещенск"},
	}}
	dicts := stubDicts{regions: map[string]int{"Амурская область": 49}}
	svc := NewLeadService(&stubLeadList{}, nil).WithDicts(dicts).WithSkorozvonMaps(maps)
	snap := entity.LeadFromSkorozvon{
		City:   "с карточки",
		Region: "Москва и Московская область",
		CustomFields: []entity.SkorozvonCustomValue{
			{FieldID: 50000009406, Raw: []byte(`[50000015671]`)},
			{FieldID: 50000011777, Raw: []byte(`[50000004560]`)},
		},
	}
	svc.enrichSnapGeo(context.Background(), &snap)
	if snap.City != "Благовещенск" {
		t.Fatalf("city %q", snap.City)
	}
	if snap.Region != "Амурская область" {
		t.Fatalf("region %q", snap.Region)
	}
	if snap.RegionID == nil || *snap.RegionID != 49 || snap.RegionSource != entity.GeoSourceSupplier {
		t.Fatalf("region_id %+v source %d", snap.RegionID, snap.RegionSource)
	}
}

func TestEnrichSnapGeoFallsBackToCallRegion(t *testing.T) {
	dicts := stubDicts{regions: map[string]int{"Москва и Московская область": 91}}
	svc := NewLeadService(&stubLeadList{}, nil).WithDicts(dicts)
	snap := entity.LeadFromSkorozvon{City: "Химки", Region: "Москва и Московская область"}
	svc.enrichSnapGeo(context.Background(), &snap)
	if snap.City != "Химки" {
		t.Fatalf("city %q", snap.City)
	}
	if snap.RegionID == nil || *snap.RegionID != 91 {
		t.Fatalf("region_id %+v", snap.RegionID)
	}
}

func TestApplyCallResultWritesMappedGeo(t *testing.T) {
	repo := &stubCallResultRepo{}
	maps := stubGeoMaps{fields: map[[2]int64]entity.SkorozvonMap{
		{50000009406, 0}:           {FieldID: 50000009406, Name: "Регион клиента", FieldType: "multiselect"},
		{50000009406, 50000015671}: {FieldID: 50000009406, OptionID: 50000015671, Name: "Амурская область"},
		{50000011777, 0}:           {FieldID: 50000011777, Name: "Город клиента", FieldType: "multiselect"},
		{50000011777, 50000004560}: {FieldID: 50000011777, OptionID: 50000004560, Name: "Благовещенск"},
	}}
	dicts := stubDicts{regions: map[string]int{"Амурская область": 49}}
	err := NewLeadService(repo, nil).WithDicts(dicts).WithSkorozvonMaps(maps).ApplyCallResult(
		context.Background(),
		"123",
		entity.LeadCallResult{CallID: "1"},
		entity.LeadFromSkorozvon{
			City:   "с карточки",
			Region: "Москва и Московская область",
			CustomFields: []entity.SkorozvonCustomValue{
				{FieldID: 50000009406, Raw: []byte(`[50000015671]`)},
				{FieldID: 50000011777, Raw: []byte(`[50000004560]`)},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if repo.geo == nil || repo.geo.City != "Благовещенск" {
		t.Fatalf("geo %+v", repo.geo)
	}
	if repo.geo.RegionID == nil || *repo.geo.RegionID != 49 || repo.geo.RegionSource != entity.GeoSourceSupplier {
		t.Fatalf("region %+v", repo.geo)
	}
	if len(repo.fields) != 2 {
		t.Fatalf("anketa fields %+v", repo.fields)
	}
	if repo.fields[0] != (entity.SkorozvonAnketaField{FieldID: 50000009406, EnumID: 50000015671}) {
		t.Fatalf("field0 %+v", repo.fields[0])
	}
	if repo.fields[1] != (entity.SkorozvonAnketaField{FieldID: 50000011777, EnumID: 50000004560}) {
		t.Fatalf("field1 %+v", repo.fields[1])
	}
}

func TestAnketaFieldsFromCustom(t *testing.T) {
	got := anketaFieldsFromCustom([]entity.SkorozvonCustomValue{
		{FieldID: 50000007376, Raw: []byte(`"Лидстех МТС"`)},
		{FieldID: 50000011777, Raw: []byte(`[50000004578, 50000004560]`)},
		{FieldID: 50000009406, Raw: []byte(`50000015671`)},
		{FieldID: 1, Raw: []byte(`"x"`)},
		{FieldID: 2, Raw: []byte(`[50000004578, 50000004578]`)},
	})
	want := []entity.SkorozvonAnketaField{
		{FieldID: 50000011777, EnumID: 50000004578},
		{FieldID: 50000011777, EnumID: 50000004560},
		{FieldID: 50000009406, EnumID: 50000015671},
		{FieldID: 2, EnumID: 50000004578},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %+v want %+v", got, want)
		}
	}
}

func TestApplyCallResultSkipsEmptyAnketaFields(t *testing.T) {
	repo := &stubCallResultRepo{}
	err := NewLeadService(repo, nil).ApplyCallResult(
		context.Background(),
		"123",
		entity.LeadCallResult{CallID: "1"},
		entity.LeadFromSkorozvon{
			CustomFields: []entity.SkorozvonCustomValue{
				{FieldID: 50000007376, Raw: []byte(`"Лидстех МТС"`)},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if repo.fields != nil {
		t.Fatalf("fields %+v", repo.fields)
	}
}

func TestFormatSkorozvonCustomFieldsUsesMapNames(t *testing.T) {
	maps := stubGeoMaps{fields: map[[2]int64]entity.SkorozvonMap{
		{50000007376, 0}:           {FieldID: 50000007376, Name: "База", FieldType: "text"},
		{50000011777, 0}:           {FieldID: 50000011777, Name: "Город клиента", FieldType: "multiselect"},
		{50000011777, 50000004578}: {FieldID: 50000011777, OptionID: 50000004578, Name: "Краснодар"},
	}}
	svc := NewLeadService(&stubLeadList{}, nil).WithSkorozvonMaps(maps)
	got := svc.formatSkorozvonCustomFields(context.Background(), []entity.SkorozvonCustomValue{
		{FieldID: 50000007376, Raw: []byte(`"Лидстех МТС"`)},
		{FieldID: 50000011777, Raw: []byte(`[50000004578]`)},
		{FieldID: 1, Raw: []byte(`"x"`)},
	})
	want := `База=Лидстех МТС,Город клиента=Краснодар,FIELD_1=x`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
