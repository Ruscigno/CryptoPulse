package feed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Ruscigno/stockscreener/models"
	"github.com/Ruscigno/stockscreener/utils"
)

const (
	FinanceYahooUrl = "https://query2.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=%d"
	PeriodString    = "&period1=%d&period2=%d"
	UserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0.4472.124"
)

type yahooScraper struct {
}

func NewYahooDataFeed() FeedConsumer {
	return &yahooScraper{}
}

// DownloadOHLCV fetches historical OHLCV data for a ticker from Yahoo Finance
func (y *yahooScraper) DownloadMarketData(symbol string, startTime time.Time, endTime *time.Time) (*models.MarketData, error) {
	url, err := y.constructURL(symbol, startTime, endTime)
	if err != nil {
		return nil, err
	}

	resp, err := y.makeHTTPRequest(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := y.checkResponseStatus(resp); err != nil {
		return nil, err
	}

	chart, err := y.parseResponse(resp)
	if err != nil {
		return nil, err
	}

	return y.extractOHLCVData(chart, symbol), nil
}

func (y *yahooScraper) constructURL(ticker string, startDate time.Time, endDate *time.Time) (string, error) {
	if endDate == nil {
		ed := time.Now().UTC()
		endDate = &ed
	}
	url := fmt.Sprintf(
		FinanceYahooUrl,
		ticker,
		int(endDate.Sub(startDate).Hours()/24)+1,
	)
	url += fmt.Sprintf(PeriodString, startDate.Unix(), endDate.Unix())
	return url, nil
}

func (y *yahooScraper) makeHTTPRequest(url string) (*http.Response, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %v", err)
	}
	return resp, nil
}

func (y *yahooScraper) checkResponseStatus(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

func (y *yahooScraper) parseResponse(resp *http.Response) (*models.YahooChartResponse, error) {
	var chart models.YahooChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&chart); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if chart.Chart.Error != nil {
		return nil, fmt.Errorf("API error: %v", chart.Chart.Error)
	}

	if len(chart.Chart.Result) == 0 || len(chart.Chart.Result[0].Timestamp) == 0 {
		return nil, fmt.Errorf("no data returned")
	}

	return &chart, nil
}

func (y *yahooScraper) extractOHLCVData(chart *models.YahooChartResponse, ticker string) *models.MarketData {
	chartResult := chart.Chart.Result[0]
	quote := chartResult.Indicators.Quote[0]
	result := &models.MarketData{
		MetaData: &models.MetaData{
			Symbol:        ticker,
			LastRefreshed: time.Unix(chartResult.Timestamp[len(chartResult.Timestamp)-1], 0).UTC(),
			TimeZone:      "UTC",
		},
	}
	result.TimeSeries = make([]*models.StockData, len(chartResult.Timestamp))
	for i := range chartResult.Timestamp {
		result.TimeSeries[i] = &models.StockData{
			Symbol:    ticker,
			CloseTime: time.Unix(chartResult.Timestamp[i], 0).UTC(),
			Open:      utils.NullToZero(quote.Open[i]),
			High:      utils.NullToZero(quote.High[i]),
			Low:       utils.NullToZero(quote.Low[i]),
			Close:     utils.NullToZero(quote.Close[i]),
			Volume:    float64(quote.Volume[i]),
		}
	}
	return result
}

func (s *yahooScraper) GetServerTimeZone() (string, error) {
	return "UTC", nil
}
