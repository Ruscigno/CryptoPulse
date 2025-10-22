package tx

import (
	"context"
	"fmt"

	"github.com/Ruscigno/CryptoPulse/pkg/wallet"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"go.uber.org/zap"
)

// TransactionFactory wraps Cosmos SDK transaction factory with dYdX-specific logic
type TransactionFactory struct {
	factory  tx.Factory
	cdc      codec.Codec
	chainID  string
	logger   *zap.Logger
	gasPrice string
	gasLimit uint64
	wallet   *wallet.Wallet
}

// NewTransactionFactory creates a new transaction factory
func NewTransactionFactory(
	cdc codec.Codec,
	chainID string,
	gasPrice string,
	gasLimit uint64,
	logger *zap.Logger,
	w *wallet.Wallet,
) *TransactionFactory {
	factory := tx.Factory{}.
		WithChainID(chainID).
		WithGasAdjustment(1.5).
		WithGasPrices(gasPrice)

	return &TransactionFactory{
		factory:  factory,
		cdc:      cdc,
		chainID:  chainID,
		logger:   logger,
		gasPrice: gasPrice,
		gasLimit: gasLimit,
		wallet:   w,
	}
}

// EstimateGas estimates gas for a transaction
func (tf *TransactionFactory) EstimateGas(
	ctx context.Context,
	msgCount int,
	fromAddress string,
	accountNumber uint64,
	sequence uint64,
) (uint64, error) {
	tf.logger.Debug("Estimating gas for transaction",
		zap.String("from", fromAddress),
		zap.Int("msg_count", msgCount))

	// For now, return a default estimate
	// In production, this would simulate the transaction on the RPC endpoint
	estimatedGas := uint64(100000) // Default estimate
	for i := 0; i < msgCount; i++ {
		estimatedGas += 50000 // Add per-message estimate
	}

	tf.logger.Debug("Gas estimation complete",
		zap.Uint64("estimated_gas", estimatedGas))

	return estimatedGas, nil
}

// GetAccountInfo retrieves account information from the chain
func (tf *TransactionFactory) GetAccountInfo(
	ctx context.Context,
	address string,
) (*authtypes.BaseAccount, error) {
	tf.logger.Debug("Fetching account info", zap.String("address", address))

	// This would require an RPC client connection
	// For now, return a placeholder
	// In production, this would query the chain

	return &authtypes.BaseAccount{
		Address:       address,
		AccountNumber: 0,
		Sequence:      0,
	}, nil
}

// GetFactory returns the underlying Cosmos SDK transaction factory
func (tf *TransactionFactory) GetFactory() tx.Factory {
	return tf.factory
}

// SetFactory updates the underlying transaction factory
func (tf *TransactionFactory) SetFactory(factory tx.Factory) {
	tf.factory = factory
}

// GetCodec returns the codec
func (tf *TransactionFactory) GetCodec() codec.Codec {
	return tf.cdc
}

// GetChainID returns the chain ID
func (tf *TransactionFactory) GetChainID() string {
	return tf.chainID
}

// GetGasPrice returns the gas price
func (tf *TransactionFactory) GetGasPrice() string {
	return tf.gasPrice
}

// GetGasLimit returns the gas limit
func (tf *TransactionFactory) GetGasLimit() uint64 {
	return tf.gasLimit
}

// SetGasLimit updates the gas limit
func (tf *TransactionFactory) SetGasLimit(gasLimit uint64) {
	tf.gasLimit = gasLimit
	tf.factory = tf.factory.WithGas(gasLimit)
}

// SignTransaction signs transaction bytes using the wallet's private key
func (tf *TransactionFactory) SignTransaction(ctx context.Context, txBytes []byte) ([]byte, error) {
	if tf.wallet == nil {
		return nil, fmt.Errorf("wallet not initialized")
	}

	tf.logger.Debug("Signing transaction", zap.Int("tx_bytes_length", len(txBytes)))

	// Sign the transaction bytes
	signature, err := tf.wallet.SignBytes(txBytes)
	if err != nil {
		tf.logger.Error("Failed to sign transaction", zap.Error(err))
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	tf.logger.Debug("Transaction signed successfully",
		zap.Int("signature_length", len(signature)))

	return signature, nil
}
