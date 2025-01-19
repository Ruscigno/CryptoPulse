package model

import "time"

type MarketData struct {
	MetaData   *MetaData    `json:"meta_data"`
	TimeSeries []*StockData `json:"time_series"`
}

type MetaData struct {
	Information   string    `json:"information"`
	Symbol        string    `json:"symbol"`
	LastRefreshed time.Time `json:"last_refreshed"`
	Interval      string    `json:"interval"`
	OutputSize    string    `json:"output_size"`
	TimeZone      string    `json:"time_zone"`
}

type StockData struct {
	Symbol string    `json:"symbol"`
	Open   float64   `json:"open"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Close  float64   `json:"close"`
	Volume int64     `json:"volume"`
	Time   time.Time `json:"timestamp"`
}
