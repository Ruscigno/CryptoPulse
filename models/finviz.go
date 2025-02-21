package models

type Rule struct {
	ID       string        `json:"id" bson:"_id"`
	Name     string        `json:"name" bson:"name"`
	Fields   []FieldRule   `json:"fields" bson:"fields"`
	NextPage *NextPageRule `json:"next_page,omitempty" bson:"next_page,omitempty"`
}

type FieldRule struct {
	Field    string `json:"field" bson:"field"`
	Selector string `json:"selector" bson:"selector"`
}

type NextPageRule struct {
	Selector string `json:"selector" bson:"selector"` // Optional: CSS selector for "Next" link
	Pattern  string `json:"pattern" bson:"pattern"`   // e.g., "r={offset}"
}

type ScrapeJob struct {
	ID      string `json:"id" bson:"_id"`
	BaseURL string `json:"base_url" bson:"base_url"`
	RuleID  string `json:"rule_id" bson:"rule_id"`
}
