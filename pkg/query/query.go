package query

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/config"
	"go.uber.org/zap"
)

// QueryClient handles queries to the dYdX Indexer API
type QueryClient struct {
	config     config.Config
	logger     *zap.Logger
	httpClient *http.Client
	baseURL    string
}

// Position represents a trading position
type Position struct {
	Market                string  `json:"market"`
	Side                  string  `json:"side"`
	Size                  string  `json:"size"`
	MaxSize               string  `json:"maxSize"`
	EntryPrice            string  `json:"entryPrice"`
	RealizedPnl           string  `json:"realizedPnl"`
	CreatedAt             string  `json:"createdAt"`
	CreatedAtHeight       string  `json:"createdAtHeight"`
	SumOpen               string  `json:"sumOpen"`
	SumClose              string  `json:"sumClose"`
	NetFunding            string  `json:"netFunding"`
	UnrealizedPnl         string  `json:"unrealizedPnl"`
	Status                string  `json:"status"`
	ClosedAt              *string `json:"closedAt,omitempty"`
	ExitPrice             *string `json:"exitPrice,omitempty"`
}

// PositionsResponse represents the response from the positions endpoint
type PositionsResponse struct {
	Positions []Position `json:"positions"`
}

// OrderResponse represents an order from the API
type OrderResponse struct {
	ID               string  `json:"id"`
	ClientID         string  `json:"clientId"`
	Market           string  `json:"market"`
	Side             string  `json:"side"`
	Size             string  `json:"size"`
	RemainingSize    string  `json:"remainingSize"`
	Price            string  `json:"price"`
	TriggerPrice     *string `json:"triggerPrice,omitempty"`
	TrailingPercent  *string `json:"trailingPercent,omitempty"`
	Type             string  `json:"type"`
	Status           string  `json:"status"`
	TimeInForce      string  `json:"timeInForce"`
	PostOnly         bool    `json:"postOnly"`
	ReduceOnly       bool    `json:"reduceOnly"`
	OrderFlags       string  `json:"orderFlags"`
	GoodTilBlock     *string `json:"goodTilBlock,omitempty"`
	GoodTilBlockTime *string `json:"goodTilBlockTime,omitempty"`
	CreatedAt        string  `json:"createdAt"`
	UnfillableAt     *string `json:"unfillableAt,omitempty"`
	UpdatedAt        string  `json:"updatedAt"`
	ClientMetadata   string  `json:"clientMetadata"`
}

// OrdersResponse represents the response from the orders endpoint
type OrdersResponse struct {
	Orders []OrderResponse `json:"orders"`
}

// MarketConfig represents market configuration
type MarketConfig struct {
	Market                string `json:"market"`
	Status                string `json:"status"`
	BaseAsset             string `json:"baseAsset"`
	QuoteAsset            string `json:"quoteAsset"`
	StepSize              string `json:"stepSize"`
	TickSize              string `json:"tickSize"`
	IndexPrice            string `json:"indexPrice"`
	OraclePrice           string `json:"oraclePrice"`
	PriceChange24H        string `json:"priceChange24H"`
	NextFundingRate       string `json:"nextFundingRate"`
	MinOrderSize          string `json:"minOrderSize"`
	Type                  string `json:"type"`
	InitialMarginFraction string `json:"initialMarginFraction"`
	MaintenanceMarginFraction string `json:"maintenanceMarginFraction"`
}

// MarketsResponse represents the response from the markets endpoint
type MarketsResponse struct {
	Markets map[string]MarketConfig `json:"markets"`
}

// NewQueryClient creates a new query client
func NewQueryClient(cfg config.Config, logger *zap.Logger) *QueryClient {
	return &QueryClient{
		config:  cfg,
		logger:  logger,
		baseURL: cfg.IndexerURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetPositions retrieves positions for a given address
func (q *QueryClient) GetPositions(ctx context.Context, address string) (*PositionsResponse, error) {
	url := fmt.Sprintf("%s/addresses/%s/subaccountNumber/0", q.baseURL, address)
	
	q.logger.Info("Fetching positions", zap.String("address", address), zap.String("url", url))
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var response PositionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	q.logger.Info("Successfully fetched positions", 
		zap.String("address", address),
		zap.Int("count", len(response.Positions)))
	
	return &response, nil
}

// GetOrders retrieves orders for a given address
func (q *QueryClient) GetOrders(ctx context.Context, address string, status *string, market *string) (*OrdersResponse, error) {
	url := fmt.Sprintf("%s/orders?address=%s&subaccountNumber=0", q.baseURL, address)
	
	if status != nil {
		url += fmt.Sprintf("&status=%s", *status)
	}
	if market != nil {
		url += fmt.Sprintf("&market=%s", *market)
	}
	
	q.logger.Info("Fetching orders", 
		zap.String("address", address),
		zap.String("url", url))
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var response OrdersResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	q.logger.Info("Successfully fetched orders", 
		zap.String("address", address),
		zap.Int("count", len(response.Orders)))
	
	return &response, nil
}

// GetOrderByID retrieves a specific order by ID
func (q *QueryClient) GetOrderByID(ctx context.Context, orderID string) (*OrderResponse, error) {
	url := fmt.Sprintf("%s/orders/%s", q.baseURL, orderID)
	
	q.logger.Info("Fetching order", zap.String("order_id", orderID))
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var response OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	q.logger.Info("Successfully fetched order", zap.String("order_id", orderID))
	
	return &response, nil
}

// GetMarkets retrieves market configuration
func (q *QueryClient) GetMarkets(ctx context.Context) (*MarketsResponse, error) {
	url := fmt.Sprintf("%s/perpetualMarkets", q.baseURL)
	
	q.logger.Info("Fetching markets", zap.String("url", url))
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var response MarketsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	q.logger.Info("Successfully fetched markets", zap.Int("count", len(response.Markets)))
	
	return &response, nil
}

// GetMarketConfig retrieves configuration for a specific market
func (q *QueryClient) GetMarketConfig(ctx context.Context, market string) (*MarketConfig, error) {
	markets, err := q.GetMarkets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch markets: %w", err)
	}
	
	config, exists := markets.Markets[market]
	if !exists {
		return nil, fmt.Errorf("market not found: %s", market)
	}
	
	return &config, nil
}
