package stockscrapper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"go.uber.org/zap"
)

var (
	INFLUXDB_TOKEN    string = os.Getenv("INFLUXDB_TOKEN")
	INFLUXDB_URL      string = os.Getenv("INFLUXDB_URL")
	INFLUXDB_USER     string = os.Getenv("INFLUXDB_USER")
	INFLUXDB_PASSWORD string = os.Getenv("INFLUXDB_PASSWORD")
)

type Server struct {
	scraper StockScrapper
}

func NewServer() *Server {
	srv := &Server{}
	srv.getServerInfo()
	srv.scraper = NewStockScrapper()
	return srv
}

func (s *Server) getServerInfo() {

}

func (s *Server) worker(client influxdb2.Client, id int, done chan bool, symbol string, timeFrame time.Duration) {
	download := func() {
		ctx := context.Background()
		err := s.scraper.DownloadMarketData(ctx, client, symbol)
		if err != nil {
			zap.L().Info("Error downloading stock data", zap.Error(err), zap.String("symbol", symbol), zap.Int("worker", id))
		}
	}
	download()
	// time before the 1st second of the next minute
	tempClock := time.NewTicker(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute).Add(1 * time.Second)))
	<-tempClock.C
	tempClock.Stop()
	clock := time.NewTicker(timeFrame)
	defer clock.Stop()
	for {
		select {
		case <-clock.C:
			download()
		case <-done:
			zap.L().Info("Worker %d: Exiting", zap.Int("worker", id))
			return
		}
	}
}

func (s *Server) Start() {
	stockList, err := readStockList(os.Getenv("STOCK_LIST_FILE"))
	if err != nil {
		zap.L().Error("Error reading stock list", zap.Error(err))
		return
	}

	client := s.connectToInfluxDB()
	if client == nil {
		return
	}
	defer client.Close()

	var wg sync.WaitGroup
	done := make(chan bool)

	// Start worker goroutines
	for i, symbol := range stockList {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.worker(client, id, done, symbol, time.Duration(1)*time.Minute)
		}(i)
	}
	// Handle signals
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for a signal
	<-sigchan
	zap.L().Info("Received termination signal. Shutting down...")

	// Signal workers to stop
	close(done)

	// Wait for workers to finish
	wg.Wait()
	zap.L().Info("All workers exited. Exiting main.")
}

func (*Server) connectToInfluxDB() influxdb2.Client {
	ctx := context.Background()
	client := influxdb2.NewClientWithOptions(INFLUXDB_URL, INFLUXDB_TOKEN, influxdb2.DefaultOptions())
	// Check if InfluxDB is newly installed by querying for existing databases
	query := fmt.Sprintf(`buckets() |> filter(fn: (r) => r.name == "%s")`, INFLUXDB_BUCKET)
	_, err := client.QueryAPI(INFLUXDB_ORG).Query(ctx, query)
	if err == nil {
		return client
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		zap.L().Error("Error querying InfluxDB", zap.Error(err))
		return nil
	}
	d, err := client.SetupWithToken(ctx,
		INFLUXDB_USER,
		INFLUXDB_PASSWORD,
		INFLUXDB_ORG,
		INFLUXDB_BUCKET,
		0, // infinite retention period
		INFLUXDB_TOKEN)

	if err != nil {
		zap.L().Fatal("Error setting up InfluxDB", zap.Error(err))
		return nil
	}
	zap.L().Info("InfluxDB is newly installed", zap.Any("auth_status", d.Auth.Status))
	return client
}

// readStockList reads a list of stock symbols from a file.
func readStockList(filename string) ([]string, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}
	type SymbolList map[string][]string
	var symbols SymbolList
	err = json.Unmarshal(byteValue, &symbols)
	if err != nil {
		return nil, err
	}
	return symbols["stock"], nil
}
