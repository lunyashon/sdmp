package entity

type SkorozvonLink struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	ProjectID      int64  `json:"project_id"`
	ChildSourceIDs []int  `json:"child_source_ids"`
	RegionIDs      []int  `json:"region_ids"`
}
