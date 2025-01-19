package feed

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Ruscigno/stockscreener/model"
)

type localDataFeed struct {
}

func NewLocalDataFeed() FeedConsumer {
	return &localDataFeed{}
}

func (s *localDataFeed) DownloadStockData(ticker string, lastestDate time.Time) (*model.MarketData, error) {
	fileName := fmt.Sprintf("feed/data/%s_%s.json", ticker, lastestDate.Format("2006-01"))
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return nil, fmt.Errorf("file %s does not exist", fileName)
	}
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	jsonData, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	data, err := parseStockData(jsonData)
	if err != nil {
		return nil, fmt.Errorf("error parsing stock data: %v", err)
	}
	return data, nil
}
