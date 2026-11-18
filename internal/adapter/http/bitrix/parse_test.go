package bitrix

import "testing"

func TestMapLeadSourceDescriptionFromFields(t *testing.T) {
	got := MapLead(map[string]any{
		"fields": map[string]any{
			"PHONE":              "79001234567",
			"SOURCE_ID":          "GDMP",
			"SOURCE_DESCRIPTION": "Яндекс.Директ",
		},
	})
	if got.Phone != "79001234567" || got.SourceID != "GDMP" || got.SourceDescription != "Яндекс.Директ" {
		t.Fatalf("got %+v", got)
	}
}

func TestMapLeadSourceDescriptionFromRoot(t *testing.T) {
	got := MapLead(map[string]any{
		"PHONE":              "79001112233",
		"SOURCE_DESCRIPTION": "VK Ads",
	})
	if got.Phone != "79001112233" || got.SourceDescription != "VK Ads" {
		t.Fatalf("got %+v", got)
	}
}

func TestMapLeadSourceDescriptionFieldsWinsOverRoot(t *testing.T) {
	got := MapLead(map[string]any{
		"SOURCE_DESCRIPTION": "root",
		"fields": map[string]any{
			"PHONE":              "79001234567",
			"SOURCE_DESCRIPTION": "fields",
		},
	})
	if got.SourceDescription != "fields" {
		t.Fatalf("got %+v", got)
	}
}
