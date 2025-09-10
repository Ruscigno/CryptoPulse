package config

import (
	"os"
)

// Config holds service configuration.
type Config struct {
	RPCURL     string
	IndexerURL string
	ChainID    string
	Mnemonic   string
	HTTPPort   string
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() Config {
	return Config{
		RPCURL:     getEnv("RPC_URL", "https://rpc.dydx.trade:443"),
		IndexerURL: getEnv("INDEXER_URL", "https://indexer.dydx.trade/v4"),
		ChainID:    getEnv("CHAIN_ID", "dydx-mainnet-1"),
		Mnemonic:   getEnv("MNEMONIC", ""),
		HTTPPort:   getEnv("HTTP_PORT", ":8080"),
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
