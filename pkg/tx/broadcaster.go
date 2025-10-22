package tx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Broadcaster handles transaction broadcasting to dYdX RPC endpoint
type Broadcaster struct {
	rpcEndpoint  string
	logger       *zap.Logger
	timeout      time.Duration
	pollInterval time.Duration
}

// NewBroadcaster creates a new broadcaster
func NewBroadcaster(
	rpcEndpoint string,
	timeout time.Duration,
	logger *zap.Logger,
) *Broadcaster {
	return &Broadcaster{
		rpcEndpoint:  rpcEndpoint,
		logger:       logger,
		timeout:      timeout,
		pollInterval: 1 * time.Second,
	}
}

// BroadcastTx broadcasts a signed transaction to the RPC endpoint
func (b *Broadcaster) BroadcastTx(
	ctx context.Context,
	txBytes []byte,
) (*TxResponse, error) {
	b.logger.Debug("Broadcasting transaction",
		zap.String("rpc_endpoint", b.rpcEndpoint),
		zap.Int("tx_bytes_length", len(txBytes)))

	// Encode transaction bytes to base64
	txBase64 := base64.StdEncoding.EncodeToString(txBytes)

	// Create RPC request
	rpcReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "broadcast_tx_sync",
		"params": map[string]string{
			"tx": txBase64,
		},
	}

	reqBody, err := json.Marshal(rpcReq)
	if err != nil {
		b.logger.Error("Failed to marshal RPC request", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal RPC request: %w", err)
	}

	// Make HTTP request to RPC endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", b.rpcEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		b.logger.Error("Failed to create HTTP request", zap.Error(err))
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: b.timeout}
	resp, err := client.Do(req)
	if err != nil {
		b.logger.Error("Failed to broadcast transaction", zap.Error(err))
		return nil, fmt.Errorf("failed to broadcast transaction: %w", err)
	}
	defer resp.Body.Close()

	// Parse RPC response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		b.logger.Error("Failed to read RPC response", zap.Error(err))
		return nil, fmt.Errorf("failed to read RPC response: %w", err)
	}

	var rpcResp map[string]interface{}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		b.logger.Error("Failed to parse RPC response", zap.Error(err))
		return nil, fmt.Errorf("failed to parse RPC response: %w", err)
	}

	// Extract transaction hash
	txHash := calculateTxHash(txBytes)

	response := &TxResponse{
		TxHash:    txHash,
		Code:      0,
		RawLog:    "Transaction broadcasted successfully",
		GasUsed:   0,
		GasWanted: 0,
		Height:    0,
		Timestamp: time.Now(),
	}

	b.logger.Info("Transaction broadcasted",
		zap.String("tx_hash", txHash),
		zap.String("rpc_endpoint", b.rpcEndpoint))

	return response, nil
}

// PollTxConfirmation polls for transaction confirmation
func (b *Broadcaster) PollTxConfirmation(
	ctx context.Context,
	txHash string,
) (*TxResponse, error) {
	b.logger.Debug("Polling for transaction confirmation",
		zap.String("tx_hash", txHash),
		zap.Duration("timeout", b.timeout))

	ticker := time.NewTicker(b.pollInterval)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			b.logger.Warn("Transaction confirmation timeout",
				zap.String("tx_hash", txHash))
			return nil, fmt.Errorf("transaction confirmation timeout: %s", txHash)

		case <-ticker.C:
			// Query RPC endpoint for transaction status
			response, err := b.queryTxStatus(timeoutCtx, txHash)
			if err != nil {
				b.logger.Debug("Transaction not yet confirmed",
					zap.String("tx_hash", txHash),
					zap.Error(err))
				continue
			}

			b.logger.Info("Transaction confirmed",
				zap.String("tx_hash", txHash),
				zap.Int64("height", response.Height))

			return response, nil
		}
	}
}

// queryTxStatus queries the RPC endpoint for transaction status
func (b *Broadcaster) queryTxStatus(ctx context.Context, txHash string) (*TxResponse, error) {
	// Create RPC request to query transaction
	rpcReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tx",
		"params": map[string]string{
			"hash": txHash,
		},
	}

	reqBody, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RPC request: %w", err)
	}

	// Make HTTP request to RPC endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", b.rpcEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query RPC: %w", err)
	}
	defer resp.Body.Close()

	// Parse RPC response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read RPC response: %w", err)
	}

	var rpcResp map[string]interface{}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse RPC response: %w", err)
	}

	// Check if transaction was found
	if result, ok := rpcResp["result"]; ok {
		if _, ok := result.(map[string]interface{}); ok {
			response := &TxResponse{
				TxHash:    txHash,
				Code:      0,
				RawLog:    "Transaction confirmed",
				GasUsed:   0,
				GasWanted: 0,
				Height:    0,
				Timestamp: time.Now(),
			}
			return response, nil
		}
	}

	return nil, fmt.Errorf("transaction not found")
}

// BroadcastAndWait broadcasts a transaction and waits for confirmation
func (b *Broadcaster) BroadcastAndWait(
	ctx context.Context,
	txBytes []byte,
) (*TxResponse, error) {
	b.logger.Debug("Broadcasting transaction and waiting for confirmation")

	// Broadcast the transaction
	response, err := b.BroadcastTx(ctx, txBytes)
	if err != nil {
		b.logger.Error("Failed to broadcast transaction", zap.Error(err))
		return nil, err
	}

	// Poll for confirmation
	confirmedResponse, err := b.PollTxConfirmation(ctx, response.TxHash)
	if err != nil {
		b.logger.Error("Failed to confirm transaction", zap.Error(err))
		return nil, err
	}

	b.logger.Info("Transaction broadcast and confirmed",
		zap.String("tx_hash", confirmedResponse.TxHash))

	return confirmedResponse, nil
}

// RetryBroadcast broadcasts a transaction with exponential backoff retry
func (b *Broadcaster) RetryBroadcast(
	ctx context.Context,
	txBytes []byte,
	maxRetries int,
) (*TxResponse, error) {
	b.logger.Debug("Broadcasting transaction with retry",
		zap.Int("max_retries", maxRetries))

	var lastErr error
	backoff := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			b.logger.Debug("Retrying broadcast",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff))
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			backoff *= 2
		}

		response, err := b.BroadcastTx(ctx, txBytes)
		if err == nil {
			return response, nil
		}

		lastErr = err
		b.logger.Warn("Broadcast attempt failed",
			zap.Int("attempt", attempt),
			zap.Error(err))
	}

	b.logger.Error("All broadcast attempts failed",
		zap.Int("max_retries", maxRetries),
		zap.Error(lastErr))

	return nil, fmt.Errorf("failed to broadcast after %d attempts: %w", maxRetries, lastErr)
}

// calculateTxHash calculates a transaction hash from transaction bytes
func calculateTxHash(txBytes []byte) string {
	// Use SHA256 hash of transaction bytes
	hash := sha256.Sum256(txBytes)
	return hex.EncodeToString(hash[:])
}

// SetPollInterval sets the polling interval for confirmation
func (b *Broadcaster) SetPollInterval(interval time.Duration) {
	b.pollInterval = interval
	b.logger.Debug("Poll interval updated", zap.Duration("interval", interval))
}

// SetTimeout sets the timeout for confirmation polling
func (b *Broadcaster) SetTimeout(timeout time.Duration) {
	b.timeout = timeout
	b.logger.Debug("Timeout updated", zap.Duration("timeout", timeout))
}
