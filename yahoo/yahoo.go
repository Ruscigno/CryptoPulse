package yahoo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OHLCV represents a single day's OHLC and volume data
type OHLCV struct {
	Date   time.Time `json:"date"`
	Open   float64   `json:"open"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Close  float64   `json:"close"`
	Volume int64     `json:"volume"`
}

// YahooChartResponse represents the structure of Yahoo's chart API response
type YahooChartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"chart"`
}

// DownloadOHLCV fetches historical OHLCV data for a ticker from Yahoo Finance
func DownloadOHLCV(ticker string, startDate, endDate time.Time) ([]OHLCV, error) {
	// Construct the Yahoo Finance API URL
	url := fmt.Sprintf(
		"https://query2.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=%d",
		ticker,
		int(endDate.Sub(startDate).Hours()/24)+1, // Days between start and end
	)

	// Add start and end dates as Unix timestamps
	url += fmt.Sprintf("&period1=%d&period2=%d", startDate.Unix(), endDate.Unix())

	// Make HTTP request
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0.4472.124")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data for %s: %v", ticker, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse JSON response
	var chart YahooChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&chart); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if chart.Chart.Error != nil {
		return nil, fmt.Errorf("API error: %v", chart.Chart.Error)
	}

	if len(chart.Chart.Result) == 0 || len(chart.Chart.Result[0].Timestamp) == 0 {
		return nil, fmt.Errorf("no data returned for %s", ticker)
	}

	// Extract OHLCV data
	result := chart.Chart.Result[0]
	quote := result.Indicators.Quote[0]
	data := make([]OHLCV, len(result.Timestamp))

	for i := range result.Timestamp {
		data[i] = OHLCV{
			Date:   time.Unix(result.Timestamp[i], 0).UTC(),
			Open:   nullToZero(quote.Open[i]),
			High:   nullToZero(quote.High[i]),
			Low:    nullToZero(quote.Low[i]),
			Close:  nullToZero(quote.Close[i]),
			Volume: quote.Volume[i],
		}
	}

	return data, nil
}

// nullToZero handles null values in Yahoo's response (sometimes returned as 0 or omitted)
func nullToZero(val float64) float64 {
	if val == 0 || val != val { // NaN check
		return 0
	}
	return val
}

// Example usage (can be called from elsewhere)
func Example() {
	ticker := "AAPL"
	startDate := time.Now().AddDate(-1, 0, 0) // 1 year ago
	endDate := time.Now()

	data, err := DownloadOHLCV(ticker, startDate, endDate)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	for _, d := range data {
		fmt.Printf("%s: O=%.2f, H=%.2f, L=%.2f, C=%.2f, V=%d\n",
			d.Date.Format("2006-01-02"), d.Open, d.High, d.Low, d.Close, d.Volume)
	}
}
