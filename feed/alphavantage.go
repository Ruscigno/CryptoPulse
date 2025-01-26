package feed

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Ruscigno/stockscreener/model"
	"go.uber.org/zap"
)

type alphaVantageScrapper struct {
}

const (
	FUNCTION          = "TIME_SERIES_INTRADAY"
	EXTENDED_HOURS    = "true" // Extended hours data
	ADJUSTED          = "true" // Adjusted data
	INTERVAL          = "1min" // Time interval for intraday data
	OUTPUT_SIZE       = "full" // Full data set
	DATA_TYPE         = "json" // Output format
	ALPHA_VANTAGE_URL = "https://www.alphavantage.co/query"
)

var (
	apiKey string = os.Getenv("ALPHA_VANTAGE_API_KEY")
)

func NewAlphaVantageScrapper() FeedConsumer {
	if apiKey == "" {
		zap.L().Fatal("Alpha Vantage API key is missing. Please set the 'apiKey' variable")
	}
	return &alphaVantageScrapper{}
}

func (s *alphaVantageScrapper) DownloadMarketData(symbol string, startTime time.Time, endTime *time.Time) (*model.MarketData, error) {
	var result *model.MarketData
	// build a list of months to download, from the lastDate to the current date
	months := buildMonthList(startTime)
	// iterate over the months and download the data
	for _, month := range months {
		monthlyData, err := s.fetchMarketData(symbol, month)
		if err != nil {
			return nil, err
		}
		if monthlyData == nil {
			continue
		}
		if monthlyData.MetaData.LastRefreshed.After(startTime) {
			startTime = monthlyData.MetaData.LastRefreshed
		}
		if result == nil {
			result = monthlyData
			continue
		}
		result.TimeSeries = append(result.TimeSeries, monthlyData.TimeSeries...)
	}
	zap.L().Info("Downloaded stock data", zap.String("symbol", symbol))
	return result, nil
}

func buildMonthList(lastDate time.Time) []time.Time {
	// get the current date
	currentDate := time.Now().UTC()

	// build a list of months to download, from the lastDate to the current date
	months := []time.Time{}
	for currentDate.After(lastDate) {
		months = append(months, currentDate)
		currentDate = currentDate.AddDate(0, -1, 0)
	}

	return months
}

func (s *alphaVantageScrapper) fetchMarketData(symbol string, month time.Time) (*model.MarketData, error) {
	// format month as "YYYY-MM"
	monthStr := month.Format("2006-01")
	// Build the URL
	queryURL := fmt.Sprintf("%s?function=%s&symbol=%s&adjusted=%s&interval=%s&outputsize=%s&datatype=%s&month=%s&apikey=%s",
		ALPHA_VANTAGE_URL, FUNCTION, symbol, ADJUSTED, INTERVAL, OUTPUT_SIZE, DATA_TYPE, monthStr, apiKey)

	// Perform the HTTP request
	resp, err := http.Get(queryURL)
	if err != nil {
		zap.L().Error("HTTP request failed", zap.Error(err))
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		zap.L().Error("Non-200 response", zap.String("status", resp.Status))
		return nil, fmt.Errorf("non-200 response: %s", resp.Status)
	}

	// save the body as csv
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Error("Failed to read response body", zap.Error(err))
		return nil, fmt.Errorf("error reading response body: %v", err)
	}
	data, err := s.ParseStockData(body)
	if err != nil {
		zap.L().Error("Error parsing stock data", zap.Error(err))
		return nil, fmt.Errorf("error parsing stock data: %v", err)
	}
	return data, nil
}

func (s *alphaVantageScrapper) GetServerTimeZone() (string, error) {
	return "UTC", nil
}
