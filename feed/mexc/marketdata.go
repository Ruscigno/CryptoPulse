package mexc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Ruscigno/stockscreener/feed"
	"github.com/Ruscigno/stockscreener/models"
	"go.uber.org/zap"
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
		zap.L().Fatal("Mexc API key is missing. Please set the 'MEXC_API_KEY' variable")
	}
	if MexcSecretKey == "" {
		zap.L().Fatal("Mexc Secret key is missing. Please set the 'MEXC_SECRET_KEY' variable")
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

func (s *mexcDataFeed) DownloadMarketData(symbol string, startTime time.Time, endTime *time.Time) (*models.MarketData, error) {
	if endTime == nil {
		now := time.Now()
		endTime = &now
	}
	result := &models.MarketData{
		MetaData: &models.MetaData{
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
		zap.L().Info("Downloaded market data", zap.String("symbol", symbol), zap.String("from", rt.startTime.Format(time.RFC3339)), zap.String("to", rt.endTime.Format(time.RFC3339)))
	}
	if !maxDate.IsZero() {
		result.MetaData.LastRefreshed = maxDate
		endTime = &maxDate
	}
	if len(result.TimeSeries) == 0 {
		endTime = &startTime
		zap.L().Warn("No data for symbol", zap.String("symbol", symbol))
		return nil, nil
	}
	return result, nil
}

// buildRequestList build a list of requests for every 500 minutes using the start and end time as interval
func buildRequestTimeList(startTime time.Time, endTime time.Time) []*timeRequestList {
	if fetchBytimeLimit >= limit {
		zap.L().Fatal("fetchBytimeLimit should be less than limit", zap.Int("fetchBytimeLimit", fetchBytimeLimit), zap.Int("limit", limit))
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

func (s *mexcDataFeed) fetchMarketData(symbol string, startTime time.Time, endTime time.Time) (*models.MarketData, error) {
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
		zap.L().Info("empty response body", zap.String("url", url), zap.String("startTime", startTime.Format(time.RFC3339)), zap.String("endTime", endTime.Format(time.RFC3339)))
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
func (s *mexcDataFeed) parseMexcResponse(symbol string, jsonData []byte) (*models.MarketData, error) {
	// Define a variable to hold the raw data
	var rawData [][]interface{}

	// Unmarshal the JSON data into rawData
	if err := json.Unmarshal(jsonData, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	// Initialize MarketData
	marketData := &models.MarketData{
		TimeSeries: make([]*models.StockData, 0, len(rawData)),
	}

	// Iterate over each entry in rawData
	for idx, entry := range rawData {
		if len(entry) < 8 {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", "insufficient data"))
			continue
		}

		// Extract and assert each field
		openTimeMs, ok := entry[0].(float64)
		if !ok {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", "invalid open_time"))
			continue
		}

		openPrice, ok := entry[1].(string)
		if !ok {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", "invalid open_price"))
			continue
		}

		highPrice, ok := entry[2].(string)
		if !ok {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", "invalid high_price"))
			continue
		}

		lowPrice, ok := entry[3].(string)
		if !ok {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", "invalid low_price"))
			continue
		}

		closePrice, ok := entry[4].(string)
		if !ok {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", "invalid close_price"))
			continue
		}

		volume, ok := entry[5].(string)
		if !ok {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", "invalid volume"))
			continue
		}

		closeTimeMs, ok := entry[6].(float64)
		if !ok {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", "invalid close_time"))
			continue
		}

		quoteVolume, ok := entry[7].(string)
		if !ok {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", "invalid quote_volume"))
			continue
		}

		// Convert timestamps from milliseconds to time.Time
		openTime := time.UnixMilli(int64(openTimeMs)).UTC()
		closeTime := time.UnixMilli(int64(closeTimeMs)).UTC()

		// Convert string prices and volumes to float64
		open, err := parseStringToFloat(openPrice)
		if err != nil {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", err.Error()))
			continue
		}

		high, err := parseStringToFloat(highPrice)
		if err != nil {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", err.Error()))
			continue
		}

		low, err := parseStringToFloat(lowPrice)
		if err != nil {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", err.Error()))
			continue
		}

		closeP, err := parseStringToFloat(closePrice)
		if err != nil {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", err.Error()))
			continue
		}

		vol, err := parseStringToFloat(volume)
		if err != nil {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", err.Error()))
			continue
		}

		qVol, err := parseStringToFloat(quoteVolume)
		if err != nil {
			zap.L().Info("Skipping entry", zap.Int("index", idx), zap.String("reason", err.Error()))
			continue
		}

		// Create models.StockData instance
		stockData := &models.StockData{
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
