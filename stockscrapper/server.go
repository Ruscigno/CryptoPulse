package stockscrapper

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	stockListFile    = "stocklist.json"
	defaultTimeFrame = "1m"
)

type Server struct {
	scraper StockScrapper
}

func NewServer() *Server {
	return &Server{
		scraper: NewStockScrapper(),
	}
}

func (s *Server) worker(id int, done chan bool, stock string, timeFrame time.Duration) {
	if 1 != 2 {
		timeFrame = time.Duration(10) * time.Second
	}
	ticker := time.NewTicker(timeFrame)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := s.scraper.DownloadStockData(stock, defaultTimeFrame)
			if err != nil {
				fmt.Printf("Worker %d: Error downloading stock data for %s: %v\n", id, stock, err)
			}
			fmt.Printf("Worker %d: Downloaded stock data for %s\n", id, stock)
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

	var wg sync.WaitGroup
	done := make(chan bool)

	// Start worker goroutines
	for i, stock := range stockList {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.worker(id, done, stock, time.Duration(1)*time.Minute)
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
