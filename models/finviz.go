package models

type Rule struct {
	ID       string        `json:"id" bson:"_id"`
	Name     string        `json:"name" bson:"name"`
	Table    TableRule     `json:"table" bson:"table"`
	NextPage *NextPageRule `json:"next_page,omitempty" bson:"next_page,omitempty"`
}

type TableRule struct {
	Selector string  `json:"selector" bson:"selector"` // e.g., "table.screener-table"
	Rows     RowRule `json:"rows" bson:"rows"`
}

type RowRule struct {
	Selector string      `json:"selector" bson:"selector"` // e.g., "tr"
	Fields   []FieldRule `json:"fields" bson:"fields"`
}

type FieldRule struct {
	Field    string `json:"field" bson:"field"`       // e.g., "ticker"
	Selector string `json:"selector" bson:"selector"` // e.g., "td:nth-child(2) a"
}

type NextPageRule struct {
	Selector string `json:"selector" bson:"selector"` // Optional: CSS selector for "Next" link
	Pattern  string `json:"pattern" bson:"pattern"`   // e.g., "r={offset}"
}

type ScrapeJob struct {
	ID              string `json:"id" bson:"_id"`
	BaseURL         string `json:"base_url" bson:"base_url"`
	RuleID          string `json:"rule_id" bson:"rule_id"`
	DomainGlobal    string `json:"domain_global" bson:"domain_global"`
	Parallelism     int    `json:"parallelism" bson:"parallelism"`
	Delay           int    `json:"delay" bson:"delay"`
	UserAgent       string `json:"user_agent" bson:"user_agent"`
	MaxDepth        int    `json:"max_depth" bson:"max_depth"`
	OffsetIncrement int    `json:"offset_increment" bson:"offset_increment"`
	OffsetMax       int    `json:"offset_max" bson:"offset_max"`
}
