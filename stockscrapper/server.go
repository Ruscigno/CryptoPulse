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
)

const (
	stockListFile = "stocklist.json"
)

var (
	INFLUXDB_TOKEN string = os.Getenv("INFLUXDB_TOKEN")
	INFLUX_URL     string = os.Getenv("INFLUX_URL")
	API_KEY        string = os.Getenv("ALPHA_VANTAGE_API_KEY")
)

type Server struct {
	scraper StockScrapper
}

func NewServer() *Server {
	return &Server{
		scraper: NewStockScrapper(API_KEY),
	}
}

func (s *Server) worker(client influxdb2.Client, id int, done chan bool, ticker string, timeFrame time.Duration) {
	if 1 != 2 {
		timeFrame = time.Duration(10) * time.Second
	}
	clock := time.NewTicker(timeFrame)
	defer clock.Stop()

	for {
		select {
		case <-clock.C:
			ctx := context.Background()
			err := s.scraper.DownloadStockData(ctx, client, ticker)
			if err != nil {
				fmt.Printf("Worker %d: Error downloading stock data for %s: %v\n", id, ticker, err)
			}
			fmt.Printf("Worker %d: Downloaded stock data for %s\n", id, ticker)
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
	for i, stock := range stockList {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.worker(client, id, done, stock, time.Duration(1)*time.Minute)
		}(i)
	}

	// Handle signals
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for a signal
	<-sigchan
	fmt.Println("Received termination signal. Shutting down...")

	// Signal workers to stop
	close(done)

	// Wait for workers to finish
	wg.Wait()
	fmt.Println("All workers exited. Exiting main.")
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
