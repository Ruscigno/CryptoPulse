package feed

import (
	"time"

	"github.com/Ruscigno/stockscreener/model"
)

const (
	DataFeedProviderLocal        = "local"
	DataFeedProviderAlphaVantage = "alphavantage"
)

type FeedConsumer interface {
	DownloadStockData(ticker string, lastestDate time.Time) (*model.MarketData, error)
}
