package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ruscigno/stock-screener/internal/config"
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

func (s *Server) handleScreen(w http.ResponseWriter, r *http.Request) {
	req := screener.Request{
		Symbols:    csvOrDefault(r.URL.Query().Get("symbols"), s.cfg.Stocks),
		Timeframes: csvOrDefault(r.URL.Query().Get("timeframes"), s.cfg.Timeframes),
		Match:      orDefault(r.URL.Query().Get("match"), s.cfg.Screening.Match),
		Indicators: csvOrDefault(r.URL.Query().Get("indicators"), screener.AllIndicators),
	}
	for _, tf := range req.Timeframes {
		if _, ok := timeframe.Get(tf); !ok {
			http.Error(w, "unknown timeframe: "+tf, http.StatusBadRequest)
			return
		}
	}
	allowed := make(map[string]bool, len(s.cfg.Stocks))
	for _, sym := range s.cfg.Stocks {
		allowed[sym] = true
	}
	for _, sym := range req.Symbols {
		if !allowed[sym] {
			http.Error(w, "unknown symbol: "+sym, http.StatusBadRequest)
			return
		}
	}
	if !validMatch(req.Match) {
		http.Error(w, "invalid match mode: "+req.Match, http.StatusBadRequest)
		return
	}

	result, err := s.scr.Screen(r.Context(), req)
	if err != nil {
		http.Error(w, "screen failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, toDTO(result, req))
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

func validMatch(m string) bool {
	if m == "any" || m == "all" {
		return true
	}
	if strings.HasPrefix(m, "min:") {
		n, err := strconv.Atoi(strings.TrimPrefix(m, "min:"))
		return err == nil && n >= 1
	}
	return false
}
