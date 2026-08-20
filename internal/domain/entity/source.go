package entity

type Source struct {
	ID         int
	ParentID   *int
	Name       string
	BitrixCode string
	Token      string
	ColdLabel  string
	BaseLabel  string
	IsActive   bool
}
