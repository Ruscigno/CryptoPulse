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
	return &mexcDataFeed{}
}

func (s *mexcDataFeed) DownloadMarketData(symbol string, startTime time.Time, endTime *time.Time) (*model.MarketData, error) {
	if endTime == nil {
		now := time.Now()
		endTime = &now
	}
	tz, err := s.GetServerTimeZone()
	if err != nil {
		log.Printf("error getting server timezone: %v\n", err)
		return nil, err
	}
	result := &model.MarketData{
		MetaData: &model.MetaData{
			Symbol:        symbol,
			LastRefreshed: *endTime,
			Interval:      interval,
			TimeZone:      tz,
		},
	}
	// build a list of months to download, from the lastDate to the current date
	requestsTimeList := buildRequestTimeList(startTime, *endTime)
	result.TimeSeries = make([]*model.StockData, len(requestsTimeList))
	// iterate over the months and download the data
	for _, rt := range requestsTimeList {
		monthlyData, err := s.fetchMarketData(symbol, rt, *endTime)
		if err != nil || monthlyData == nil {
			continue
		}
		if monthlyData.MetaData.LastRefreshed.After(*endTime) {
			endTime = &monthlyData.MetaData.LastRefreshed
		}
		if result == nil {
			result = monthlyData
			continue
		}
		result.TimeSeries = append(result.TimeSeries, monthlyData.TimeSeries...)
	}
	if result == nil {
		log.Printf("No data for %s\n", symbol)
		return nil, nil
	}
	log.Printf("Downloaded market data for %s\n", symbol)
	return result, nil
}

// buildRequestList build a list of requests for every 500 minutes using the start and end time as interval
func buildRequestTimeList(startTime time.Time, endTime time.Time) []time.Time {
	if fetchBytimeLimit >= limit {
		log.Fatalf("fetchBytimeLimit [%d] should be less than limit [%d]", fetchBytimeLimit, limit)
	}
	var requests []time.Time
	for t := endTime; t.After(startTime.Add(-time.Minute * time.Duration(fetchBytimeLimit))); t = t.Add(-time.Minute * time.Duration(fetchBytimeLimit)) {
		requests = append(requests, t.Add(-time.Minute*time.Duration(fetchBytimeLimit)))
	}
	return requests
}

func (s *mexcDataFeed) fetchMarketData(symbol string, startTime time.Time, endTime time.Time) (*model.MarketData, error) {
	method := "GET"

	client := &http.Client{}
	url := fmt.Sprintf(FetchMarketDataURL, BaseURL, symbol, interval, startTime.UnixMilli(), endTime.UnixMilli(), limit)
	res, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(res)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response: %s", resp.Status)
	}
	if res.Body == nil {
		log.Printf("empty response body, url, startTime, endTime: %s, %s\n", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
		return nil, fmt.Errorf("empty response body")
	}
	defer res.Body.Close()

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
			Volume:    int64(vol),
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
