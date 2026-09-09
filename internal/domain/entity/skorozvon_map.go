package entity

import "time"

type SkorozvonMap struct {
	FieldID    int64
	OptionID   int64
	Name       string
	BitrixCode string
	FieldType  string
	LeadType   string
	UpdatedAt  time.Time
}
