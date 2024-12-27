package stockscrapper

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

const (
	FIELD_INFORMATION    = "1. Information"
	FIELD_SYMBOL         = "2. Symbol"
	FIELD_LAST_REFRESHED = "3. Last Refreshed"
	FIELD_INTERVAL       = "4. Interval"
	FIELD_OUTPUT_SIZE    = "5. Output Size"
	FIELD_TIME_ZONE      = "6. Time Zone"
	FIELD_OPEN           = "1. open"
	FIELD_HIGH           = "2. high"
	FIELD_LOW            = "3. low"
	FIELD_CLOSE          = "4. close"
	FIELD_VOLUME         = "5. volume"
)

type APIResponse struct {
	MetaData   map[string]string            `json:"Meta Data"`
	TimeSeries map[string]map[string]string `json:"Time Series (1min)"`
}

func parseStockData(jsonData []byte) (*AlphaVantageMarketData, error) {
	var response APIResponse
	if err := json.Unmarshal(jsonData, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	alphavantage := AlphaVantageMarketData{
		MetaData: &MetaData{
			Information: response.MetaData[FIELD_INFORMATION],
			Symbol:      response.MetaData[FIELD_SYMBOL],
			LastRefreshed: func() time.Time {
				t, _ := time.Parse("2006-01-02 15:04:05", response.MetaData[FIELD_LAST_REFRESHED])
				return t
			}(),
			Interval:   response.MetaData[FIELD_INTERVAL],
			OutputSize: response.MetaData[FIELD_OUTPUT_SIZE],
			TimeZone:   response.MetaData[FIELD_TIME_ZONE],
		},
	}
	for timestamp, data := range response.TimeSeries {
		open, err := strconv.ParseFloat(data[FIELD_OPEN], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse open value: %v", err)
		}

		high, err := strconv.ParseFloat(data[FIELD_HIGH], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse high value: %v", err)
		}

		low, err := strconv.ParseFloat(data[FIELD_LOW], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse low value: %v", err)
		}

		close, err := strconv.ParseFloat(data[FIELD_CLOSE], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse close value: %v", err)
		}

		volume, err := strconv.ParseInt(data[FIELD_VOLUME], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse volume value: %v", err)
		}

		t, err := time.Parse("2006-01-02 15:04:05", timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %v", err)
		}
		// set the proper timezone from alphavantage.MetaData.TimeZone
		t = t.In(time.FixedZone(alphavantage.MetaData.TimeZone, 0))

		stockData := &StockData{
			Symbol: alphavantage.MetaData.Symbol,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: volume,
			Time:   t,
		}

		alphavantage.TimeSeries = append(alphavantage.TimeSeries, stockData)
	}

	return &alphavantage, nil
}
