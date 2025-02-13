package stockscrapper

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"go.uber.org/zap"
)

const (
	stockListFile = "stocklist.json"
)

var (
	INFLUXDB_TOKEN string = os.Getenv("INFLUXDB_TOKEN")
	INFLUX_URL     string = os.Getenv("INFLUX_URL")
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
	stockList, err := readStockList(stockListFile)
	if err != nil {
		zap.L().Error("Error reading stock list", zap.Error(err))
		return
	}

	client := influxdb2.NewClient(INFLUX_URL, INFLUXDB_TOKEN)
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
