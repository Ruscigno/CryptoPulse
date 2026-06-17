package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/match"
	"github.com/Ruscigno/stock-screener/internal/screener"
	"github.com/Ruscigno/stock-screener/internal/timeframe"
)

// ScreenRunner is the screener dependency (real or fake).
type ScreenRunner interface {
	Screen(ctx context.Context, req screener.Request) (screener.Result, error)
}

// Pinger checks backing-store health.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Server struct {
	scr ScreenRunner
	db  Pinger
	cfg *config.Config
}

func NewServer(scr ScreenRunner, db Pinger, cfg *config.Config) *Server {
	return &Server{scr: scr, db: db, cfg: cfg}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/screen", s.handleScreen)
	mux.HandleFunc("/healthz", s.handleHealthz)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// reqError is a client-facing request error with an HTTP status.
type reqError struct {
	status int
	msg    string
}

// parseRequest builds a screener.Request from the query string (each param
// defaulting to config) and validates it. Shared by /screen and /matches.
func (s *Server) parseRequest(r *http.Request) (screener.Request, *reqError) {
	req := screener.Request{
		Symbols:    csvOrDefault(r.URL.Query().Get("symbols"), s.cfg.Stocks),
		Timeframes: csvOrDefault(r.URL.Query().Get("timeframes"), s.cfg.Timeframes),
		Match:      orDefault(r.URL.Query().Get("match"), s.cfg.Screening.Match),
		Indicators: csvOrDefault(r.URL.Query().Get("indicators"), screener.AllIndicators),
	}
	for _, tf := range req.Timeframes {
		if _, ok := timeframe.Get(tf); !ok {
			return req, &reqError{http.StatusBadRequest, "unknown timeframe: " + tf}
		}
	}
	allowed := make(map[string]bool, len(s.cfg.Stocks))
	for _, sym := range s.cfg.Stocks {
		allowed[sym] = true
	}
	for _, sym := range req.Symbols {
		if !allowed[sym] {
			return req, &reqError{http.StatusBadRequest, "unknown symbol: " + sym}
		}
	}
	if !match.Valid(req.Match) {
		return req, &reqError{http.StatusBadRequest, "invalid match mode: " + req.Match}
	}
	if err := validateIndicators(req.Indicators); err != nil {
		return req, &reqError{http.StatusBadRequest, err.Error()}
	}
	return req, nil
}

func (s *Server) handleScreen(w http.ResponseWriter, r *http.Request) {
	req, rerr := s.parseRequest(r)
	if rerr != nil {
		http.Error(w, rerr.msg, rerr.status)
		return
	}
	result, err := s.scr.Screen(r.Context(), req)
	if err != nil {
		// Log the detail server-side; don't leak internals to the client.
		log.Printf("screen failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, toDTO(result, req))
}

// validateIndicators rejects unknown or duplicate indicator names, bounding the
// per-request work (consistent with the symbols/timeframes validation).
func validateIndicators(inds []string) error {
	allowed := make(map[string]bool, len(screener.AllIndicators))
	for _, name := range screener.AllIndicators {
		allowed[name] = true
	}
	seen := make(map[string]bool, len(inds))
	for _, name := range inds {
		if !allowed[name] {
			return fmt.Errorf("unknown indicator: %s", name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate indicator: %s", name)
		}
		seen[name] = true
	}
	return nil
}

// ---- DTOs (stable JSON shape, decoupled from internal types) ----

type pivotDTO struct {
	Value float64   `json:"value"`
	Time  time.Time `json:"time"`
}
type indicatorDTO struct {
	Latest    float64    `json:"latest"`
	Trend     string     `json:"trend"`
	Zone      string     `json:"zone"`
	Triggered bool       `json:"triggered"`
	Peaks     []pivotDTO `json:"peaks"`
	Valleys   []pivotDTO `json:"valleys"`
}
type rowDTO struct {
	Symbol     string                  `json:"symbol"`
	Timeframe  string                  `json:"timeframe"`
	BarTime    time.Time               `json:"bar_time"`
	Price      float64                 `json:"price"`
	Triggered  []string                `json:"triggered"`
	Indicators map[string]indicatorDTO `json:"indicators"`
}
type warningDTO struct {
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"`
	Message   string `json:"message"`
}
type responseDTO struct {
	AsOf     time.Time `json:"as_of"`
	Criteria struct {
		Match      string   `json:"match"`
		Symbols    int      `json:"symbols"`
		Timeframes []string `json:"timeframes"`
	} `json:"criteria"`
	Results  []rowDTO     `json:"results"`
	Warnings []warningDTO `json:"warnings"`
}

func toDTO(res screener.Result, req screener.Request) responseDTO {
	var out responseDTO
	out.AsOf = time.Now().UTC()
	out.Criteria.Match = req.Match
	out.Criteria.Symbols = len(req.Symbols)
	out.Criteria.Timeframes = req.Timeframes
	out.Results = make([]rowDTO, 0, len(res.Rows))
	for _, row := range res.Rows {
		rd := rowDTO{
			Symbol: row.Symbol, Timeframe: row.Timeframe, BarTime: row.BarTime,
			Price: row.Price, Triggered: row.Triggered,
			Indicators: map[string]indicatorDTO{},
		}
		for name, ir := range row.Indicators {
			rd.Indicators[name] = indicatorDTO{
				Latest: ir.Latest, Trend: ir.Trend, Zone: ir.Zone, Triggered: ir.Triggered,
				Peaks: pivotsToDTO(ir.Peaks), Valleys: pivotsToDTO(ir.Valleys),
			}
		}
		out.Results = append(out.Results, rd)
	}
	out.Warnings = make([]warningDTO, 0, len(res.Warnings))
	for _, wn := range res.Warnings {
		out.Warnings = append(out.Warnings, warningDTO(wn))
	}
	return out
}

func pivotsToDTO(in []screener.PivotPoint) []pivotDTO {
	out := make([]pivotDTO, 0, len(in))
	for _, p := range in {
		out = append(out, pivotDTO{Value: p.Value, Time: p.Time})
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func csvOrDefault(s string, def []string) []string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
