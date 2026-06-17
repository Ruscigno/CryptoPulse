package yahoo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const baseURL = "https://query1.finance.yahoo.com/v8/finance/chart/"

// Candle is one OHLCV row from Yahoo.
type Candle struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

type Client struct {
	http      *http.Client
	userAgent string
}

func New() *Client {
	return &Client{
		http:      &http.Client{Timeout: 30 * time.Second},
		userAgent: "Mozilla/5.0 (stock-screener)",
	}
}

// rangeFor returns Yahoo's default range string for an interval when no
// explicit start time is given.
func rangeFor(interval string) string {
	switch interval {
	case "15m", "30m":
		return "60d"
	case "60m", "90m", "1h":
		return "730d"
	default: // 1d, 1wk, 1mo
		return "max"
	}
}

// Fetch returns candles for a symbol at a Yahoo interval. If `from` is non-zero
// it requests period1=from..now (incremental); otherwise it uses the default
// range for the interval.
func (c *Client) Fetch(ctx context.Context, symbol, interval string, from time.Time) ([]Candle, error) {
	q := url.Values{}
	q.Set("interval", interval)
	if from.IsZero() {
		q.Set("range", rangeFor(interval))
	} else {
		q.Set("period1", fmt.Sprintf("%d", from.Unix()))
		q.Set("period2", fmt.Sprintf("%d", time.Now().Unix()))
	}
	u := baseURL + url.PathEscape(symbol) + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo %s: status %d", symbol, resp.StatusCode)
	}
	return parseChart(body)
}

type chartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

func parseChart(raw []byte) ([]Candle, error) {
	var cr chartResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("decode chart: %w", err)
	}
	if cr.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo error: %s", cr.Chart.Error.Description)
	}
	if len(cr.Chart.Result) == 0 || len(cr.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, fmt.Errorf("yahoo: empty result")
	}
	res := cr.Chart.Result[0]
	q := res.Indicators.Quote[0]
	var out []Candle
	for i, ts := range res.Timestamp {
		if i >= len(q.Open) || i >= len(q.High) || i >= len(q.Low) || i >= len(q.Close) {
			break
		}
		o, h, l, cl := q.Open[i], q.High[i], q.Low[i], q.Close[i]
		if o == nil || h == nil || l == nil || cl == nil {
			continue // gap row
		}
		vol := 0.0
		if i < len(q.Volume) && q.Volume[i] != nil {
			vol = *q.Volume[i]
		}
		out = append(out, Candle{
			Time:  time.Unix(ts, 0).UTC(),
			Open:  *o, High: *h, Low: *l, Close: *cl, Volume: vol,
		})
	}
	return out, nil
}
