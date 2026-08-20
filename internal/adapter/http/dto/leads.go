package dto

import "github.com/lunyashon/sdmp/internal/domain/entity"

type SourceDTO struct {
	ID         int    `json:"id"`
	ParentID   *int   `json:"parent_id"`
	Name       string `json:"name"`
	BitrixCode string `json:"bitrix_code"`
	ColdLabel  string `json:"cold_label,omitempty"`
	BaseLabel  string `json:"base_label,omitempty"`
	IsActive   bool   `json:"is_active"`
}

type SourcesResponse struct {
	Count   int         `json:"count"`
	Sources []SourceDTO `json:"sources"`
}

func NewSourceDTO(s entity.Source) SourceDTO {
	return SourceDTO{
		ID:         s.ID,
		ParentID:   s.ParentID,
		Name:       s.Name,
		BitrixCode: s.BitrixCode,
		ColdLabel:  s.ColdLabel,
		BaseLabel:  s.BaseLabel,
		IsActive:   s.IsActive,
	}
}

type LeadDTO struct {
	Phone      string `json:"phone"`
	SourceID   string `json:"source_id"`
	Region     string `json:"region"`
	Operator   string `json:"operator"`
	City       string `json:"city"`
	Name       string `json:"name"`
	OwnerID    string `json:"owner_id"`
	DateCreate string `json:"date_create"`
}

type LeadsQuery struct {
	SourceID string `form:"source_id" binding:"required"`
	DateFrom string `form:"date_from" binding:"required"`
	DateTo   string `form:"date_to" binding:"required"`
}

type LeadsResponse struct {
	SourceID      string    `json:"source_id"`
	DateFrom      string    `json:"date_from"`
	DateTo        string    `json:"date_to"`
	MonthsLoaded  []string  `json:"months_loaded"`
	MonthsMissing []string  `json:"months_missing"`
	Count         int       `json:"count"`
	Leads         []LeadDTO `json:"leads"`
}

func NewLeadDTO(l entity.LeadRecord) LeadDTO {
	return LeadDTO{
		Phone:      l.Phone,
		SourceID:   l.SourceID,
		Region:     l.Region,
		Operator:   l.Operator,
		City:       l.City,
		Name:       l.Name,
		OwnerID:    l.OwnerID,
		DateCreate: l.DateCreate,
	}
}
