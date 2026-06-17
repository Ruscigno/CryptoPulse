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

const defaultBaseURL = "https://query1.finance.yahoo.com/v8/finance/chart/"

// maxResponseBytes caps the chart response read to guard against unbounded
// bodies from this unofficial upstream (OOM protection).
const maxResponseBytes = 25 << 20 // 25 MiB

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
	http        *http.Client
	userAgent   string
	baseURL     string
	maxAttempts int
	baseDelay   time.Duration
}

func New() *Client {
	return &Client{
		http:        &http.Client{Timeout: 30 * time.Second},
		userAgent:   "Mozilla/5.0 (stock-screener)",
		baseURL:     defaultBaseURL,
		maxAttempts: 3,
		baseDelay:   500 * time.Millisecond,
	}
}

// rangeFor returns Yahoo's `range` string for an intraday interval, or "" for
// daily-and-longer intervals. Daily+ intentionally returns "" because Yahoo's
// `range=max` silently coarsens 1d/1wk/1mo to quarterly bars; those intervals
// must be fetched via period1/period2 instead (see Fetch), which preserves the
// requested granularity.
func rangeFor(interval string) string {
	switch interval {
	case "15m", "30m":
		return "60d"
	case "60m", "90m", "1h":
		return "730d"
	default: // 1d, 1wk, 1mo, 3mo
		return ""
	}
}

// Fetch returns candles for a symbol at a Yahoo interval. If `from` is non-zero
// it requests period1=from..now (incremental); otherwise it uses the default
// range for the interval. Transient failures (transport errors, HTTP 429, and
// 5xx) are retried with exponential backoff up to maxAttempts; the backoff
// respects ctx cancellation.
func (c *Client) Fetch(ctx context.Context, symbol, interval string, from time.Time) ([]Candle, error) {
	q := url.Values{}
	q.Set("interval", interval)
	switch {
	case !from.IsZero():
		// Incremental fetch from the last stored bar.
		q.Set("period1", fmt.Sprintf("%d", from.Unix()))
		q.Set("period2", fmt.Sprintf("%d", time.Now().Unix()))
	case rangeFor(interval) != "":
		// Intraday: range-based (Yahoo caps intraday history anyway).
		q.Set("range", rangeFor(interval))
	default:
		// Daily+: full history via period1/period2. NOT range=max — that makes
		// Yahoo coarsen 1d/1wk/1mo to quarterly bars.
		q.Set("period1", "0")
		q.Set("period2", fmt.Sprintf("%d", time.Now().Unix()))
	}
	u := c.baseURL + url.PathEscape(symbol) + "?" + q.Encode()

	var lastErr error
	for attempt := 0; attempt < c.maxAttempts; attempt++ {
		if attempt > 0 {
			delay := c.baseDelay << (attempt - 1) // exponential backoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		candles, retryable, err := c.attempt(ctx, u, symbol)
		if err == nil {
			return candles, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, fmt.Errorf("yahoo %s: giving up after %d attempts: %w", symbol, c.maxAttempts, lastErr)
}

// attempt performs one HTTP request. retryable is true for failures worth
// retrying (transport error, HTTP 429, HTTP 5xx).
func (c *Client) attempt(ctx context.Context, u, symbol string) (candles []Candle, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err // transport/network error
	}
	defer resp.Body.Close()
	// Cap the body: this is an unofficial upstream, so a hostile/compromised
	// response must not be able to exhaust memory.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, true, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("yahoo %s: status %d", symbol, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("yahoo %s: status %d", symbol, resp.StatusCode)
	}
	candles, err = parseChart(body)
	if err != nil {
		return nil, false, err
	}
	return candles, false, nil
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
			Time: time.Unix(ts, 0).UTC(),
			Open: *o, High: *h, Low: *l, Close: *cl, Volume: vol,
		})
	}
	return out, nil
}
