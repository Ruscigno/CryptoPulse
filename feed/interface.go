package feed

import (
	"time"

	"github.com/Ruscigno/stockscreener/models"
)

const (
	DataFeedProviderLocal        = "local"
	DataFeedProviderAlphaVantage = "alphavantage"
	DataFeedProviderMEXC         = "mexc"
	DataFeedProviderYahoo        = "yahoo"
)

type FeedConsumer interface {
	DownloadMarketData(symbol string, startTime time.Time, endTime *time.Time) (*models.MarketData, error)
	GetServerTimeZone() (string, error)
}
