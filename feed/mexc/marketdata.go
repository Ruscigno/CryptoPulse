package mexc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Ruscigno/stockscreener/feed"
	"github.com/Ruscigno/stockscreener/model"
)

type mexcDataFeed struct {
	client *http.Client
}

type timeRequestList struct {
	startTime time.Time
	endTime   time.Time
}

const (
	MexcFunction           = "TIME_SERIES_INTRADAY"
	interval               = "1m"
	limit              int = 900
	fetchBytimeLimit   int = 500
	Timeout            int = 10
	MaxRetries         int = 3
	BaseURL                = "https://api.mexc.com"
	FetchMarketDataURL     = "%s/api/v3/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=%d"
	fetchServerTimeURL     = "/api/v3/time"
)

var (
	MexcApiKey    string = os.Getenv("MEXC_API_KEY")
	MexcSecretKey string = os.Getenv("MEXC_SECRET_KEY")
	tz            string = ""
)

func NewMexcDataFeed() feed.FeedConsumer {
	if MexcApiKey == "" {
		log.Fatal("Mexc API key is missing. Please set the 'MEXC_API_KEY' variable.")
	}
	if MexcSecretKey == "" {
		log.Fatal("Mexc Secret key is missing. Please set the 'MEXC_SECRET_KEY' variable.")
	}
	return &mexcDataFeed{
		client: &http.Client{
			Timeout: time.Duration(Timeout) * time.Second * 5,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 2,
				DisableKeepAlives:   true,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

func (s *mexcDataFeed) DownloadMarketData(symbol string, startTime time.Time, endTime *time.Time) (*model.MarketData, error) {
	if endTime == nil {
		now := time.Now()
		endTime = &now
	}
	result := &model.MarketData{
		MetaData: &model.MetaData{
			Symbol:        symbol,
			LastRefreshed: time.Time{},
			Interval:      interval,
			TimeZone:      tz,
		},
	}
	// build a list of months to download, from the lastDate to the current date
	requestsTimeList := buildRequestTimeList(startTime, *endTime)
	maxDate := time.Time{}
	// iterate over the months and download the data
	for _, rt := range requestsTimeList {
		periodData, err := s.fetchMarketData(symbol, rt.startTime, rt.endTime)
		if err != nil || periodData == nil {
			continue
		}
		if rt.endTime.After(maxDate) {
			maxDate = rt.endTime
		}
		result.TimeSeries = append(result.TimeSeries, periodData.TimeSeries...)
		log.Printf("Downloaded market data for %s, from %s to %s\n", symbol, rt.startTime.Format(time.RFC3339), rt.endTime.Format(time.RFC3339))
	}
	if !maxDate.IsZero() {
		result.MetaData.LastRefreshed = maxDate
		endTime = &maxDate
	}
	if len(result.TimeSeries) == 0 {
		endTime = &startTime
		log.Printf("No data for %s\n", symbol)
		return nil, nil
	}
	log.Printf("Download market data finished for %s\n", symbol)
	return result, nil
}

// buildRequestList build a list of requests for every 500 minutes using the start and end time as interval
func buildRequestTimeList(startTime time.Time, endTime time.Time) []*timeRequestList {
	if fetchBytimeLimit >= limit {
		log.Fatalf("fetchBytimeLimit [%d] should be less than limit [%d]", fetchBytimeLimit, limit)
	}
	var requests []*timeRequestList
	// truncate endTime to the minute
	endTime = endTime.Truncate(time.Minute)
	for startTime.Before(endTime) {
		st := endTime.Add(-time.Minute * time.Duration(fetchBytimeLimit))
		if st.Before(startTime) {
			st = startTime
		}
		requests = append(requests, &timeRequestList{
			startTime: st,
			endTime:   endTime,
		})
		endTime = st
	}
	return requests
}

func (s *mexcDataFeed) fetchMarketData(symbol string, startTime time.Time, endTime time.Time) (*model.MarketData, error) {
	defer s.client.CloseIdleConnections()

	url := fmt.Sprintf(FetchMarketDataURL, BaseURL, symbol, interval, startTime.UnixMilli(), endTime.UnixMilli(), limit)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response: %s", resp.Status)
	}
	if resp.Body == nil {
		log.Printf("empty response body, url, startTime, endTime: %s, %s\n", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
		return nil, fmt.Errorf("empty response body")
	}
	defer resp.Body.Close()

	// save the body as csv
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}
	data, err := s.parseMexcResponse(symbol, body)
	if err != nil {
		return nil, fmt.Errorf("error parsing stock data: %v", err)
	}

	return data, nil
}

// ParseMexcResponse parses the MEXC BTCUSDT price data JSON response.
// It returns a MarketData instance containing a slice of KlineData.
func (s *mexcDataFeed) parseMexcResponse(symbol string, jsonData []byte) (*model.MarketData, error) {
	// Define a variable to hold the raw data
	var rawData [][]interface{}

	// Unmarshal the JSON data into rawData
	if err := json.Unmarshal(jsonData, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	// Initialize MarketData
	marketData := &model.MarketData{
		TimeSeries: make([]*model.StockData, 0, len(rawData)),
	}

	// Iterate over each entry in rawData
	for idx, entry := range rawData {
		if len(entry) < 8 {
			log.Printf("Skipping entry %d: insufficient data", idx)
			continue
		}

		// Extract and assert each field
		openTimeMs, ok := entry[0].(float64)
		if !ok {
			log.Printf("Skipping entry %d: invalid open_time", idx)
			continue
		}

		openPrice, ok := entry[1].(string)
		if !ok {
			log.Printf("Skipping entry %d: invalid open price", idx)
			continue
		}

		highPrice, ok := entry[2].(string)
		if !ok {
			log.Printf("Skipping entry %d: invalid high price", idx)
			continue
		}

		lowPrice, ok := entry[3].(string)
		if !ok {
			log.Printf("Skipping entry %d: invalid low price", idx)
			continue
		}

		closePrice, ok := entry[4].(string)
		if !ok {
			log.Printf("Skipping entry %d: invalid close price", idx)
			continue
		}

		volume, ok := entry[5].(string)
		if !ok {
			log.Printf("Skipping entry %d: invalid volume", idx)
			continue
		}

		closeTimeMs, ok := entry[6].(float64)
		if !ok {
			log.Printf("Skipping entry %d: invalid close_time", idx)
			continue
		}

		quoteVolume, ok := entry[7].(string)
		if !ok {
			log.Printf("Skipping entry %d: invalid quote_volume", idx)
			continue
		}

		// Convert timestamps from milliseconds to time.Time
		openTime := time.UnixMilli(int64(openTimeMs)).UTC()
		closeTime := time.UnixMilli(int64(closeTimeMs)).UTC()

		// Convert string prices and volumes to float64
		open, err := parseStringToFloat(openPrice)
		if err != nil {
			log.Printf("Skipping entry %d: %v", idx, err)
			continue
		}

		high, err := parseStringToFloat(highPrice)
		if err != nil {
			log.Printf("Skipping entry %d: %v", idx, err)
			continue
		}

		low, err := parseStringToFloat(lowPrice)
		if err != nil {
			log.Printf("Skipping entry %d: %v", idx, err)
			continue
		}

		closeP, err := parseStringToFloat(closePrice)
		if err != nil {
			log.Printf("Skipping entry %d: %v", idx, err)
			continue
		}

		vol, err := parseStringToFloat(volume)
		if err != nil {
			log.Printf("Skipping entry %d: %v", idx, err)
			continue
		}

		qVol, err := parseStringToFloat(quoteVolume)
		if err != nil {
			log.Printf("Skipping entry %d: %v", idx, err)
			continue
		}

		// Create model.StockData instance
		stockData := &model.StockData{
			Symbol:    symbol,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closeP,
			Volume:    vol,
			QuoteVol:  qVol,
			OpenTime:  openTime,
			CloseTime: closeTime,
		}
		marketData.TimeSeries = append(marketData.TimeSeries, stockData)
	}
	return marketData, nil
}

// parseStringToFloat converts a string to float64.
func parseStringToFloat(s string) (float64, error) {
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0, fmt.Errorf("invalid float string '%s': %v", s, err)
	}
	return f, nil
}
