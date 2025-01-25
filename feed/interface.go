package feed

import (
	"time"

	"github.com/Ruscigno/stockscreener/model"
)

const (
	DataFeedProviderLocal        = "local"
	DataFeedProviderAlphaVantage = "alphavantage"
	DataFeedProviderMEXC         = "mexc"
)

type FeedConsumer interface {
	DownloadMarketData(symbol string, startTime time.Time, endTime *time.Time) (*model.MarketData, error)
	GetServerTimeZone() (string, error)
}
