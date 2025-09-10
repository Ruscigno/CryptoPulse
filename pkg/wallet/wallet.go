package wallet

import (
	"context"

	"github.com/Ruscigno/CryptoPulse/pkg/config"
)

// Wallet manages key derivation and signing.
type Wallet struct {
	cfg config.Config
}

// NewWallet creates a new Wallet instance.
func NewWallet(cfg config.Config) *Wallet {
	return &Wallet{cfg: cfg}
}

// GetAddress returns the wallet address (placeholder).
func (w *Wallet) GetAddress(ctx context.Context) (string, error) {
	return "dydx1...", nil // Implement key derivation later
}
