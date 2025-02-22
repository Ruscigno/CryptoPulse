package feed

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Ruscigno/stockscreener/models"
	"go.uber.org/zap"
)

type localDataFeed struct {
}

func NewLocalDataFeed() FeedConsumer {
	return &localDataFeed{}
}

func (s *localDataFeed) DownloadMarketData(symbol string, startTime time.Time, endTime *time.Time) (*models.MarketData, error) {
	fileName := fmt.Sprintf("feed/data/%s_%s.json", symbol, startTime.Format("2006-01"))
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		zap.L().Error("file does not exist", zap.String("file", fileName))
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
	av := alphaVantageScrapper{}
	data, err := av.ParseStockData(jsonData)
	if err != nil {
		zap.L().Error("error parsing stock data", zap.Error(err))
		return nil, fmt.Errorf("error parsing stock data: %v", err)
	}
	return data, nil
}

func (s *localDataFeed) GetServerTimeZone() (string, error) {
	return "UTC", nil
}
