package entity

type SplitRule struct {
	ID                int    `json:"id"`
	OperatorID        int    `json:"operator_id"`
	ChildSourceID     int    `json:"child_source_id"`
	TestChildSourceID *int   `json:"test_child_source_id"`
	ColdLabel         string `json:"cold_label"`
	BaseLabel         string `json:"base_label"`
}

type SourceSplit struct {
	SourceID         int         `json:"source_id"`
	TegActive        bool        `json:"teg_active"`
	TegExpectedValue string      `json:"teg_expected_value"`
	Rules            []SplitRule `json:"rules"`
}

type SplitResolve struct {
	SplitRule
	TegActive        bool
	TegExpectedValue string
	TestColdLabel    string
	TestBaseLabel    string
}
