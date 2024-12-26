package stockscrapper

type stockScrapper struct {
}

func NewStockScrapper() StockScrapper {
	return &stockScrapper{}
}

func (s *stockScrapper) DownloadStockData(symbol string, timeFrame string) error {
	return nil
}
