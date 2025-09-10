package wallet

import (
	"context"
	"fmt"
	"os"

	"github.com/Ruscigno/CryptoPulse/pkg/config"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"
	"go.uber.org/zap"
)

// Wallet manages key derivation and signing for dYdX transactions
type Wallet struct {
	cfg        config.Config
	logger     *zap.Logger
	privateKey *secp256k1.PrivKey
	publicKey  *secp256k1.PubKey
	address    string
	mnemonic   string
	hdPath     string
}

// WalletConfig holds wallet-specific configuration
type WalletConfig struct {
	Mnemonic string
	HDPath   string
	Prefix   string
}

// NewWallet creates a new Wallet instance with proper key derivation
func NewWallet(cfg config.Config, logger *zap.Logger) (*Wallet, error) {
	walletCfg := WalletConfig{
		Mnemonic: cfg.Mnemonic,
		HDPath:   getEnvString("WALLET_HD_PATH", "m/44'/118'/0'/0/0"),
		Prefix:   "dydx",
	}

	if walletCfg.Mnemonic == "" {
		return nil, fmt.Errorf("mnemonic is required")
	}

	wallet := &Wallet{
		cfg:      cfg,
		logger:   logger,
		mnemonic: walletCfg.Mnemonic,
		hdPath:   walletCfg.HDPath,
	}

	if err := wallet.deriveKeys(); err != nil {
		return nil, fmt.Errorf("failed to derive keys: %w", err)
	}

	logger.Info("Wallet initialized",
		zap.String("address", wallet.address),
		zap.String("hd_path", wallet.hdPath))

	return wallet, nil
}

// deriveKeys derives the private and public keys from the mnemonic
func (w *Wallet) deriveKeys() error {
	// Validate mnemonic
	if !bip39.IsMnemonicValid(w.mnemonic) {
		return fmt.Errorf("invalid mnemonic")
	}

	// Generate seed from mnemonic
	seed, err := bip39.NewSeedWithErrorChecking(w.mnemonic, "")
	if err != nil {
		return fmt.Errorf("failed to generate seed: %w", err)
	}

	// Derive master key
	masterPriv, ch := hd.ComputeMastersFromSeed(seed)

	// Parse HD path
	hdPath, err := hd.NewParamsFromPath(w.hdPath)
	if err != nil {
		return fmt.Errorf("failed to parse HD path: %w", err)
	}

	// Derive private key
	derivedPriv, err := hd.DerivePrivateKeyForPath(masterPriv, ch, hdPath.String())
	if err != nil {
		return fmt.Errorf("failed to derive private key: %w", err)
	}

	// Create secp256k1 private key
	w.privateKey = &secp256k1.PrivKey{Key: derivedPriv}
	w.publicKey = w.privateKey.PubKey().(*secp256k1.PubKey)

	// Generate address
	w.address, err = w.generateAddress()
	if err != nil {
		return fmt.Errorf("failed to generate address: %w", err)
	}

	return nil
}

// generateAddress generates the dYdX address from the public key
func (w *Wallet) generateAddress() (string, error) {
	// Set the address prefix for dYdX
	config := types.GetConfig()
	config.SetBech32PrefixForAccount("dydx", "dydxpub")
	config.SetBech32PrefixForValidator("dydxvaloper", "dydxvaloperpub")
	config.SetBech32PrefixForConsensusNode("dydxvalcons", "dydxvalconspub")

	// Generate address from public key
	addr := types.AccAddress(w.publicKey.Address())
	return addr.String(), nil
}

// GetAddress returns the wallet address
func (w *Wallet) GetAddress(ctx context.Context) (string, error) {
	return w.address, nil
}

// GetPublicKey returns the public key
func (w *Wallet) GetPublicKey() *secp256k1.PubKey {
	return w.publicKey
}

// GetPrivateKey returns the private key (use with caution)
func (w *Wallet) GetPrivateKey() *secp256k1.PrivKey {
	return w.privateKey
}

// SignBytes signs the given bytes with the wallet's private key
func (w *Wallet) SignBytes(data []byte) ([]byte, error) {
	if w.privateKey == nil {
		return nil, fmt.Errorf("private key not initialized")
	}

	signature, err := w.privateKey.Sign(data)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	return signature, nil
}

// SignHash signs the given hash with the wallet's private key
func (w *Wallet) SignHash(hash []byte) ([]byte, error) {
	return w.SignBytes(hash)
}

// VerifySignature verifies a signature against the wallet's public key
func (w *Wallet) VerifySignature(data, signature []byte) bool {
	if w.publicKey == nil {
		return false
	}

	return w.publicKey.VerifySignature(data, signature)
}

// GetAccountInfo returns account information for the wallet
func (w *Wallet) GetAccountInfo() map[string]interface{} {
	return map[string]interface{}{
		"address":  w.address,
		"hd_path":  w.hdPath,
		"has_keys": w.privateKey != nil,
	}
}

// Helper function to get environment variables with default values
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
