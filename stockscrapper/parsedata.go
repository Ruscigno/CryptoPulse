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
	TIME_LAYOUT          = "2006-01-02 15:04:05"
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
	if response.MetaData == nil {
		return nil, nil
	}
	if response.TimeSeries == nil {
		return nil, nil
	}

	alphavantage := AlphaVantageMarketData{
		MetaData: &MetaData{
			Information: response.MetaData[FIELD_INFORMATION],
			Symbol:      response.MetaData[FIELD_SYMBOL],
			Interval:    response.MetaData[FIELD_INTERVAL],
			OutputSize:  response.MetaData[FIELD_OUTPUT_SIZE],
			TimeZone:    response.MetaData[FIELD_TIME_ZONE],
		},
	}
	var err error
	alphavantage.MetaData.LastRefreshed, err = parseTime(response.MetaData[FIELD_LAST_REFRESHED], alphavantage.MetaData.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("failed to parse last refreshed time: %v", err)
	}

	alphavantage.MetaData.LastRefreshed = alphavantage.MetaData.LastRefreshed.In(time.FixedZone(alphavantage.MetaData.TimeZone, 0))
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

		t, err := parseTime(timestamp, alphavantage.MetaData.TimeZone)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %v", err)
		}

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

func parseTime(timestamp string, timeZone string) (time.Time, error) {
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		fmt.Printf("Error loading timezone: %v\n", err)
		return time.Time{}, err
	}

	// Parse the time string in the specified timezone
	parsedTime, err := time.ParseInLocation(TIME_LAYOUT, timestamp, location)
	if err != nil {
		fmt.Printf("Error parsing time: %v\n", err)
		return time.Time{}, err
	}
	return parsedTime, nil
}
