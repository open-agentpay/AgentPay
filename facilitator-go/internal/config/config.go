package config

import (
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config encapsulates configuration for the Facilitator server.
type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`

	EVM struct {
		RPCURL           string `yaml:"rpc_url"`
		ChainID          int64  `yaml:"chain_id"`
		TokenAddress     string `yaml:"token_address"`
		PrivateKey       string `yaml:"private_key"`
		AutoPollReceipts bool   `yaml:"auto_poll_receipts"`
	} `yaml:"evm"`

	Logger struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logger"`
}

// DefaultConfig returns default production/testnet settings.
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8001

	cfg.EVM.RPCURL = "https://sepolia-rollup.arbitrum.io/rpc"
	cfg.EVM.ChainID = 421614
	cfg.EVM.TokenAddress = "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d" // Arbitrum Sepolia USDC
	cfg.EVM.AutoPollReceipts = true

	cfg.Logger.Level = "info"
	cfg.Logger.Format = "json"
	return cfg
}

// LoadConfig loads configuration from a YAML file if provided, applying environment variable overrides.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse yaml config: %w", err)
		}
	}

	// Environment variable overrides
	if host := os.Getenv("FACILITATOR_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Server.Port = p
		}
	}
	if portStr := os.Getenv("FACILITATOR_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Server.Port = p
		}
	}

	if rpc := os.Getenv("RPC_URL"); rpc != "" {
		cfg.EVM.RPCURL = rpc
	}
	if cid := os.Getenv("CHAIN_ID"); cid != "" {
		if c, err := strconv.ParseInt(cid, 10, 64); err == nil {
			cfg.EVM.ChainID = c
		}
	}
	if token := os.Getenv("USDC_ADDRESS"); token != "" {
		cfg.EVM.TokenAddress = token
	}
	if key := os.Getenv("FACILITATOR_PRIVATE_KEY"); key != "" {
		cfg.EVM.PrivateKey = key
	}

	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		cfg.Logger.Level = strings.ToLower(lvl)
	}
	if fmt := os.Getenv("LOG_FORMAT"); fmt != "" {
		cfg.Logger.Format = strings.ToLower(fmt)
	}

	return cfg, nil
}

func (c *Config) BigChainID() *big.Int {
	return big.NewInt(c.EVM.ChainID)
}
