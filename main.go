package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"
	quote "github.com/markcheno/go-quote"
)

type StockConfig struct {
	Stocks []string `json:"stocks"`
}

var db *sql.DB

func initDB() {
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	if dbUser == "" || dbPassword == "" || dbHost == "" || dbName == "" {
		log.Fatalf("Database credentials are not fully set in environment variables.")
	}

	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Database is not reachable: %v", err)
	}

	fmt.Println("Database connected successfully")
}

func fetchAndStoreStockData(stockSymbol string) {
	// Fetch stock data using the go-quote library
	data, err := quote.NewQuoteFromYahoo(stockSymbol, "2024-01-01", "2024-12-23", "1m", true)
	if err != nil {
		log.Printf("Failed to fetch data for %s: %v", stockSymbol, err)
		return
	}

	if len(data.Close) == 0 {
		log.Printf("No data found for stock: %s", stockSymbol)
		return
	}

	// Use the latest data point
	latestIndex := len(data.Close) - 1
	timestamp := data.Date[latestIndex]
	open := data.Open[latestIndex]
	high := data.High[latestIndex]
	low := data.Low[latestIndex]
	close := data.Close[latestIndex]
	volume := float64(data.Volume[latestIndex])

	query := `INSERT INTO intraday_prices (timestamp, open, high, low, close, volume, stock_symbol)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = db.Exec(query, timestamp, open, high, low, close, volume, stockSymbol)
	if err != nil {
		log.Printf("Failed to insert data for %s: %v", stockSymbol, err)
		return
	}

	fmt.Printf("Data inserted for stock: %s\n", stockSymbol)
}

func worker(stockQueue chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for stock := range stockQueue {
		fetchAndStoreStockData(stock)
		// Re-enqueue the stock
		stockQueue <- stock
		// Wait before processing again
		time.Sleep(60 * time.Second)
	}
}

func loadStockConfig(filePath string) (*StockConfig, error) {
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config StockConfig
	err = json.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <path_to_stocks.json>", os.Args[0])
	}

	stockFile := os.Args[1]
	config, err := loadStockConfig(stockFile)
	if err != nil {
		log.Fatalf("Failed to load stock config: %v", err)
	}

	if len(config.Stocks) == 0 {
		log.Fatalf("No stocks found in the configuration file.")
	}

	initDB()
	defer db.Close()

	stockQueue := make(chan string, len(config.Stocks))
	for _, stock := range config.Stocks {
		stockQueue <- stock
	}

	var wg sync.WaitGroup
	workerCount := 4

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker(stockQueue, &wg)
	}

	wg.Wait()
}
