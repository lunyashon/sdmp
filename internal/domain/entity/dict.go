package entity

type NamedRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type OperatorGroup struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}
