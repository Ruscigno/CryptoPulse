package stockscrapper

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Ruscigno/stockscreener/feed"
	"github.com/Ruscigno/stockscreener/feed/mexc"
	"github.com/Ruscigno/stockscreener/model"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"go.uber.org/zap"
)

type StockScrapper interface {
	DownloadMarketData(ctx context.Context, client influxdb2.Client, symbol string) error
}

type stockScrapper struct {
	lastestDate time.Time
	influx      influxdb2.Client
	feed        feed.FeedConsumer
}

var (
	INFLUXDB_ORG       string = os.Getenv("INFLUXDB_ORG")
	INFLUXDB_BUCKET    string = os.Getenv("INFLUXDB_BUCKET")
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
	case feed.DataFeedProviderMEXC:
		return &stockScrapper{
			feed: mexc.NewMexcDataFeed(),
		}
	default:
		zap.L().Fatal("Unsupported data feed provider", zap.String("provider", DATA_FEED_PROVIDER))
		return nil
	}
}

func (s *stockScrapper) DownloadMarketData(ctx context.Context, client influxdb2.Client, symbol string) error {
	if client == nil {
		return fmt.Errorf("influxdb client is nil")
	}
	_, err := s.feed.GetServerTimeZone()
	if err != nil {
		zap.L().Fatal("error getting server timezone", zap.Error(err))
	}
	var lastestDate time.Time
	mu := &sync.Mutex{}

	if s.influx == nil {
		mu.Lock()
		s.influx = client
		mu.Unlock()
	}
	s.lastestDate = s.getLastDate(ctx, symbol)
	lastestDate = s.lastestDate

	data, err := s.feed.DownloadMarketData(symbol, s.lastestDate, nil)
	if err != nil {
		return err
	}
	if data != nil && data.MetaData == nil && data.TimeSeries == nil {
		zap.L().Warn("No data for symbol", zap.String("symbol", symbol))
		return nil
	}
	if data == nil || len(data.TimeSeries) == 0 {
		return nil
	}

	err = s.storeStockData(ctx, data)
	if err != nil {
		zap.L().Error("Error storing stock data", zap.Error(err))
	}
	if data.MetaData.LastRefreshed.After(lastestDate) {
		lastestDate = data.MetaData.LastRefreshed
	}
	if !lastestDate.IsZero() {
		mu.Lock()
		s.lastestDate = lastestDate
		mu.Unlock()
	}
	return nil
}

func (s *stockScrapper) getLastDate(ctx context.Context, symbol string) time.Time {
	if !s.lastestDate.IsZero() {
		return s.lastestDate
	}
	mu := &sync.Mutex{}
	mu.Lock()
	defer mu.Unlock()

	// query influxdb for the last date
	query := fmt.Sprintf(`from(bucket:"%s")|> range(start: -1y) |> filter(fn: (r) => r._measurement == "stock_data" and r.symbol == "%s") |> last()`, INFLUXDB_BUCKET, symbol)
	result, err := s.influx.QueryAPI(INFLUXDB_ORG).Query(ctx, query)
	if err != nil {
		zap.L().Fatal("Error querying influxdb", zap.Error(err))
		return time.Time{}
	}
	defer result.Close()
	for result.Next() {
		record := result.Record()
		if record == nil {
			continue
		}
		if s.lastestDate.Before(record.Time()) {
			s.lastestDate = record.Time()
		}
	}
	if s.lastestDate.IsZero() {
		s.lastestDate = time.Now().AddDate(0, 0, -365)
	}
	return s.lastestDate.Add(-time.Hour * 24)
}

func (s *stockScrapper) storeStockData(ctx context.Context, data *model.MarketData) error {
	if data == nil || data.TimeSeries == nil {
		return nil
	}
	writeAPI := s.influx.WriteAPIBlocking(INFLUXDB_ORG, INFLUXDB_BUCKET)
	for _, stockData := range data.TimeSeries {
		p := influxdb2.NewPointWithMeasurement("stock_data").
			AddTag("symbol", stockData.Symbol).
			AddField("open", stockData.Open).
			AddField("high", stockData.High).
			AddField("low", stockData.Low).
			AddField("close", stockData.Close).
			AddField("volume", stockData.Volume).
			AddField("open_time", stockData.OpenTime).
			AddField("close_time", stockData.CloseTime).
			AddField("quote_volume", stockData.QuoteVol).
			AddField("time_zone", data.MetaData.TimeZone).
			SetTime(stockData.OpenTime)
		err := writeAPI.WritePoint(context.Background(), p)
		if err != nil {
			return fmt.Errorf("influx error, writing point: %v", err)
		}
	}
	return writeAPI.Flush(ctx)
}
