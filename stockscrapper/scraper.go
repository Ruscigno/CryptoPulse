package stockscrapper

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Ruscigno/stockscreener/feed"
	"github.com/Ruscigno/stockscreener/model"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

type StockScrapper interface {
	DownloadStockData(ctx context.Context, client influxdb2.Client, symbol string) error
}

type stockScrapper struct {
	lastestDate time.Time
	influx      influxdb2.Client
	feed        feed.FeedConsumer
}

const (
	DEFAULT_TIME_ZONE = "US/Eastern"
)

var (
	INFLUX_ORG         string = os.Getenv("INFLUX_ORG")
	INFLUX_BUCKET      string = os.Getenv("INFLUX_BUCKET")
	DATA_FEED_PROVIDER string = os.Getenv("DATA_FEED_PROVIDER")
)

func NewStockScrapper() StockScrapper {
	switch DATA_FEED_PROVIDER {
	case feed.DataFeedProviderLocal:
		return &stockScrapper{
			feed: feed.NewLocalDataFeed(),
		}
	case feed.DataFeedProviderAlphaVantage:
		return &stockScrapper{
			feed: feed.NewAlphaVantageScrapper(),
		}
	default:
		log.Fatalf("Unsupported data feed provider: %s", DATA_FEED_PROVIDER)
		return nil
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
		monthlyData, err := s.feed.DownloadStockData(ticker, month)
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

func (s *stockScrapper) storeStockData(ctx context.Context, data *model.MarketData) error {
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
