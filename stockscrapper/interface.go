package stockscrapper

type StockScrapper interface {
	DownloadStockData(symbol string, timeFrame string) error
}
