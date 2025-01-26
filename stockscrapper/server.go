package stockscrapper

import (
	"context"
	"encoding/json"
	"fmt"
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
	clock := time.NewTicker(timeFrame)
	defer clock.Stop()

	download := func() {
		ctx := context.Background()
		err := s.scraper.DownloadMarketData(ctx, client, symbol)
		if err != nil {
			fmt.Printf("Worker %d: Error downloading stock data for %s: %v\n", id, symbol, err)
		}
		zap.L().Info("Downloaded stock data", zap.String("symbol", symbol))
	}
	download()
	for {
		select {
		case <-clock.C:
			download()
		case <-done:
			fmt.Printf("Worker %d: Exiting\n", id)
			return
		}
	}
}

func (s *Server) Start() {
	stockList, err := readStockList(stockListFile)
	if err != nil {
		fmt.Printf("Error reading stock list: %v\n", err)
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
