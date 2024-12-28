package stockscrapper

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

type StockScrapper interface {
	DownloadStockData(ctx context.Context, client influxdb2.Client, symbol string) error
}

type stockScrapper struct {
	apiKey      string
	lastestDate time.Time
	influx      influxdb2.Client
}

type AlphaVantageMarketData struct {
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

const (
	FUNCTION          = "TIME_SERIES_INTRADAY"
	EXTENDED_HOURS    = "true" // Extended hours data
	ADJUSTED          = "true" // Adjusted data
	INTERVAL          = "1min" // Time interval for intraday data
	OUTPUT_SIZE       = "full" // Full data set
	DATA_TYPE         = "json" // Output format
	ALPHA_VANTAGE_URL = "https://www.alphavantage.co/query"
	DEFAULT_TIME_ZONE = "US/Eastern"
)

var (
	INFLUX_ORG    string = os.Getenv("INFLUX_ORG")
	INFLUX_BUCKET string = os.Getenv("INFLUX_BUCKET")
)

func NewStockScrapper(apiKey string) StockScrapper {
	if apiKey == "" {
		log.Fatal("Alpha Vantage API key is missing. Please set the 'apiKey' variable.")
	}
	return &stockScrapper{
		apiKey: apiKey,
	}
}

func (s *stockScrapper) DownloadStockData(ctx context.Context, client influxdb2.Client, ticker string) error {
	if client == nil {
		return fmt.Errorf("influxdb client is nil")
	}
	s.influx = client
	s.lastestDate = s.getLastDate(ctx, ticker)
	lastestDate := time.Time{}

	// build a list of months to download, from the lastDate to the current date
	months := buildMonths(s.lastestDate)
	// iterate over the months and download the data
	for _, month := range months {
		monthlyData, err := s.downloadStockData(ticker, month)
		if err != nil {
			return err
		}
		if monthlyData == nil {
			continue
		}
		err = s.storeStockData(ctx, monthlyData)
		if err != nil {
			log.Printf("Error storing stock data: %v\n", err)
		}
		log.Printf("Downloaded stock data for %s:%s\n", ticker, month.Format("2006-01"))
		if monthlyData.MetaData.LastRefreshed.After(lastestDate) {
			lastestDate = monthlyData.MetaData.LastRefreshed
		}
	}
	if !lastestDate.IsZero() {
		s.lastestDate = lastestDate
	}
	log.Printf("Downloaded stock data for %s, latest date:%s\n", ticker, s.lastestDate.Format("2006-01-02"))
	return nil
}

func buildMonths(lastDate time.Time) []time.Time {
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

func (s *stockScrapper) downloadStockData(symbol string, month time.Time) (*AlphaVantageMarketData, error) {
	// format month as "YYYY-MM"
	monthStr := month.Format("2006-01")
	// Build the URL
	queryURL := fmt.Sprintf("%s?function=%s&symbol=%s&adjusted=%s&interval=%s&outputsize=%s&datatype=%s&month=%s&apikey=%s",
		ALPHA_VANTAGE_URL, FUNCTION, symbol, ADJUSTED, INTERVAL, OUTPUT_SIZE, DATA_TYPE, monthStr, s.apiKey)

	// Perform the HTTP request
	resp, err := http.Get(queryURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response: %s", resp.Status)
	}

	// save the body as csv
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}
	data, err := parseStockData(body)
	if err != nil {
		return nil, fmt.Errorf("error parsing stock data: %v", err)
	}
	return data, nil
}

func (s *stockScrapper) getLastDate(ctx context.Context, symbol string) time.Time {
	if !s.lastestDate.IsZero() {
		return s.lastestDate
	}
	// query influxdb for the last date
	query := fmt.Sprintf(`from(bucket:"%s")|> range(start: -1y) |> filter(fn: (r) => r._measurement == "stock_data" and r.symbol == "%s") |> last()`, INFLUX_BUCKET, symbol)
	result, err := s.influx.QueryAPI(INFLUX_ORG).Query(ctx, query)
	if err != nil {
		log.Printf("Error querying influxdb: %v\n", err)
		return time.Time{}
	}
	defer result.Close()
	timeZone := DEFAULT_TIME_ZONE
	for result.Next() {
		record := result.Record()
		if record == nil {
			continue
		}
		if s.lastestDate.Before(record.Time()) {
			s.lastestDate = record.Time()
		}
		if record.Field() == "time_zone" {
			timeZone = record.Value().(string)
			break
		}
	}
	if s.lastestDate.IsZero() {
		s.lastestDate = time.Now().AddDate(0, 0, -365)
	}
	s.lastestDate = s.lastestDate.In(time.FixedZone(timeZone, 0))
	return s.lastestDate
}

func (s *stockScrapper) storeStockData(ctx context.Context, data *AlphaVantageMarketData) error {
	writeAPI := s.influx.WriteAPIBlocking(INFLUX_ORG, INFLUX_BUCKET)
	for _, stockData := range data.TimeSeries {
		if stockData.Time.Before(s.lastestDate) {
			continue
		}
		p := influxdb2.NewPointWithMeasurement("stock_data").
			AddTag("symbol", stockData.Symbol).
			AddField("open", stockData.Open).
			AddField("high", stockData.High).
			AddField("low", stockData.Low).
			AddField("close", stockData.Close).
			AddField("volume", stockData.Volume).
			AddField("time_zone", data.MetaData.TimeZone).
			SetTime(stockData.Time)
		err := writeAPI.WritePoint(context.Background(), p)
		if err != nil {
			return fmt.Errorf("influx error, writing point: %v", err)
		}
	}
	return writeAPI.Flush(ctx)
}
