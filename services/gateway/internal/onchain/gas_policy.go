package onchain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// GasMode defines the supported Gas settlement models in AgentPay.
type GasMode string

const (
	// GasModeSubsidized: Mode 1 - Seller/Facilitator absorbs 100% of EVM execution Gas (Buyer pays zero gas).
	GasModeSubsidized GasMode = "subsidized"

	// GasModeDynamicSurcharge: Mode 2 - Buyer reimburses Gas as a dynamic surcharge added to the 402 challenge amount.
	GasModeDynamicSurcharge GasMode = "dynamic_surcharge"

	// GasModeBatchAggregated: Mode 3 - Off-chain aggregation and batch multicall settlement, amortizing Gas.
	GasModeBatchAggregated GasMode = "batch_aggregated"
)

var (
	ErrGasSurchargeExceedsLimit = errors.New("GAS_SURCHARGE_EXCEEDS_LIMIT")
)

// NetworkGasPolicy defines Gas policy settings for a specific blockchain network.
type NetworkGasPolicy struct {
	Mode                GasMode `yaml:"mode" json:"mode"`
	SlippageBufferPct   float64 `yaml:"slippage_buffer_pct" json:"slippage_buffer_pct"`
	MaxAcceptableGasUSD float64 `yaml:"max_acceptable_gas_usd" json:"max_acceptable_gas_usd"`
	FixedGasLimit       uint64  `yaml:"fixed_gas_limit" json:"fixed_gas_limit"`
	BatchAmortizeFactor int     `yaml:"batch_amortize_factor" json:"batch_amortize_factor"`
}

// GasPolicyConfig holds global and per-network Gas policies loaded from gateway.yaml.
type GasPolicyConfig struct {
	DefaultMode         GasMode                     `yaml:"default_mode" json:"default_mode"`
	SlippageBufferPct   float64                     `yaml:"slippage_buffer_pct" json:"slippage_buffer_pct"`
	MaxAcceptableGasUSD float64                     `yaml:"max_acceptable_gas_usd" json:"max_acceptable_gas_usd"`
	Networks            map[string]NetworkGasPolicy `yaml:"networks" json:"networks"`
}

// DefaultGasPolicyConfig returns standard defaults (Subsidized default for seamless onboarding).
func DefaultGasPolicyConfig() GasPolicyConfig {
	return GasPolicyConfig{
		DefaultMode:         GasModeSubsidized,
		SlippageBufferPct:   10.0,
		MaxAcceptableGasUSD: 0.05,
		Networks: map[string]NetworkGasPolicy{
			DefaultOnchainNetwork: {
				Mode:                GasModeDynamicSurcharge,
				SlippageBufferPct:   10.0,
				MaxAcceptableGasUSD: 0.05,
				BatchAmortizeFactor: 10,
			},
			"eip155:84532": {
				Mode:                GasModeSubsidized,
				SlippageBufferPct:   10.0,
				MaxAcceptableGasUSD: 0.05,
				BatchAmortizeFactor: 10,
			},
		},
	}
}

type gasCacheEntry struct {
	surchargeBaseUnits int64
	costUSD            float64
	cachedAt           time.Time
}

// PricingBreakdown details the components of the 402 challenge amount.
type PricingBreakdown struct {
	BaseAmount         int64   `json:"base_amount"`
	GasSurcharge       int64   `json:"gas_surcharge"`
	TotalAmount        int64   `json:"total_amount"`
	GasMode            GasMode `json:"gas_mode"`
	EstimatedGasUSD    float64 `json:"estimated_gas_usd,omitempty"`
	SlippageBufferPct  float64 `json:"slippage_buffer_pct,omitempty"`
}

// GasPricingEngine evaluates gas policies and computes dynamic amounts without modifying client SDKs.
type GasPricingEngine struct {
	mu             sync.RWMutex
	config         GasPolicyConfig
	facilitatorURL string
	apiKey         string
	httpClient     *http.Client
	cache          map[string]gasCacheEntry
	cacheTTL       time.Duration
}

// NewGasPricingEngine creates an initialized GasPricingEngine.
func NewGasPricingEngine(cfg GasPolicyConfig, facilitatorURL, apiKey string) *GasPricingEngine {
	if cfg.DefaultMode == "" {
		cfg.DefaultMode = GasModeSubsidized
	}
	if cfg.SlippageBufferPct <= 0 {
		cfg.SlippageBufferPct = 10.0
	}
	if cfg.MaxAcceptableGasUSD <= 0 {
		cfg.MaxAcceptableGasUSD = 0.05
	}
	if cfg.Networks == nil {
		cfg.Networks = make(map[string]NetworkGasPolicy)
	}

	return &GasPricingEngine{
		config:         cfg,
		facilitatorURL: strings.TrimRight(facilitatorURL, "/"),
		apiKey:         strings.TrimSpace(apiKey),
		httpClient:     &http.Client{Timeout: 3 * time.Second},
		cache:          make(map[string]gasCacheEntry),
		cacheTTL:       15 * time.Second, // 15-second cache to prevent spamming facilitator on high QPS
	}
}

// GetPolicyForNetwork retrieves the effective policy for a target network ID.
func (e *GasPricingEngine) GetPolicyForNetwork(network string) NetworkGasPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cleanNet := strings.ToLower(strings.TrimSpace(network))
	if pol, exists := e.config.Networks[cleanNet]; exists && pol.Mode != "" {
		if pol.SlippageBufferPct <= 0 {
			pol.SlippageBufferPct = e.config.SlippageBufferPct
		}
		if pol.MaxAcceptableGasUSD <= 0 {
			pol.MaxAcceptableGasUSD = e.config.MaxAcceptableGasUSD
		}
		if pol.BatchAmortizeFactor <= 0 {
			pol.BatchAmortizeFactor = 10
		}
		return pol
	}

	return NetworkGasPolicy{
		Mode:                e.config.DefaultMode,
		SlippageBufferPct:   e.config.SlippageBufferPct,
		MaxAcceptableGasUSD: e.config.MaxAcceptableGasUSD,
		BatchAmortizeFactor: 10,
	}
}

// CalculateChallengePrice calculates the effective total amount and pricing breakdown.
func (e *GasPricingEngine) CalculateChallengePrice(ctx context.Context, network, asset string, basePrice int64) (int64, *PricingBreakdown, error) {
	policy := e.GetPolicyForNetwork(network)

	switch policy.Mode {
	case GasModeDynamicSurcharge:
		surcharge, costUSD, err := e.fetchGasEstimate(ctx, network, asset, policy.SlippageBufferPct)
		if err != nil {
			// Fallback: if facilitator gas query fails, proceed gracefully with subsidized base price
			return basePrice, &PricingBreakdown{
				BaseAmount:   basePrice,
				GasSurcharge: 0,
				TotalAmount:  basePrice,
				GasMode:      GasModeSubsidized,
			}, nil
		}

		if policy.MaxAcceptableGasUSD > 0 && costUSD > policy.MaxAcceptableGasUSD {
			return 0, nil, fmt.Errorf("%w: gas cost $%.4f exceeds ceiling $%.4f", ErrGasSurchargeExceedsLimit, costUSD, policy.MaxAcceptableGasUSD)
		}

		total := basePrice + surcharge
		return total, &PricingBreakdown{
			BaseAmount:        basePrice,
			GasSurcharge:      surcharge,
			TotalAmount:       total,
			GasMode:           GasModeDynamicSurcharge,
			EstimatedGasUSD:   costUSD,
			SlippageBufferPct: policy.SlippageBufferPct,
		}, nil

	case GasModeBatchAggregated:
		surcharge, costUSD, err := e.fetchGasEstimate(ctx, network, asset, policy.SlippageBufferPct)
		if err != nil {
			return basePrice, &PricingBreakdown{
				BaseAmount:   basePrice,
				GasSurcharge: 0,
				TotalAmount:  basePrice,
				GasMode:      GasModeBatchAggregated,
			}, nil
		}

		factor := int64(policy.BatchAmortizeFactor)
		if factor <= 0 {
			factor = 10
		}
		amortizedSurcharge := surcharge / factor
		if amortizedSurcharge < 1 {
			amortizedSurcharge = 1
		}
		total := basePrice + amortizedSurcharge

		return total, &PricingBreakdown{
			BaseAmount:        basePrice,
			GasSurcharge:      amortizedSurcharge,
			TotalAmount:       total,
			GasMode:           GasModeBatchAggregated,
			EstimatedGasUSD:   costUSD / float64(factor),
			SlippageBufferPct: policy.SlippageBufferPct,
		}, nil

	case GasModeSubsidized:
		fallthrough
	default:
		return basePrice, &PricingBreakdown{
			BaseAmount:   basePrice,
			GasSurcharge: 0,
			TotalAmount:  basePrice,
			GasMode:      GasModeSubsidized,
		}, nil
	}
}

func (e *GasPricingEngine) fetchGasEstimate(ctx context.Context, network, asset string, slippage float64) (int64, float64, error) {
	cacheKey := fmt.Sprintf("%s:%s", network, asset)
	now := time.Now()

	e.mu.RLock()
	if cached, ok := e.cache[cacheKey]; ok && now.Sub(cached.cachedAt) < e.cacheTTL {
		e.mu.RUnlock()
		return cached.surchargeBaseUnits, cached.costUSD, nil
	}
	e.mu.RUnlock()

	if e.facilitatorURL == "" {
		// Mock baseline fallback: 0.001 Gwei on L2 ~= 500 base units ($0.0005)
		return 500, 0.0005, nil
	}

	url := fmt.Sprintf("%s/v1/gas/estimate?network=%s&asset=%s&slippage=%.1f", e.facilitatorURL, network, asset, slippage)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, err
	}
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
		req.Header.Set("X-Facilitator-Key", e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("facilitator returned status %d: %s", resp.StatusCode, string(b))
	}

	var res struct {
		SurchargeBaseUnits  int64  `json:"surchargeBaseUnits"`
		EstimatedGasCostUSD string `json:"estimatedGasCostUsd"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, 0, err
	}

	var costUSD float64
	_, _ = fmt.Sscanf(res.EstimatedGasCostUSD, "%f", &costUSD)

	e.mu.Lock()
	e.cache[cacheKey] = gasCacheEntry{
		surchargeBaseUnits: res.SurchargeBaseUnits,
		costUSD:            costUSD,
		cachedAt:           now,
	}
	e.mu.Unlock()

	return res.SurchargeBaseUnits, costUSD, nil
}

// LoadGasPolicyFromYAML parses the gas_policy section of a gateway YAML configuration file.
func LoadGasPolicyFromYAML(path string) GasPolicyConfig {
	cfg := DefaultGasPolicyConfig()
	if path == "" {
		return cfg
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	var raw struct {
		Settlement struct {
			GasPolicy GasPolicyConfig `yaml:"gas_policy"`
		} `yaml:"settlement"`
	}

	if err := yaml.Unmarshal(data, &raw); err == nil && raw.Settlement.GasPolicy.DefaultMode != "" {
		parsed := raw.Settlement.GasPolicy
		if parsed.SlippageBufferPct <= 0 {
			parsed.SlippageBufferPct = cfg.SlippageBufferPct
		}
		if parsed.MaxAcceptableGasUSD <= 0 {
			parsed.MaxAcceptableGasUSD = cfg.MaxAcceptableGasUSD
		}
		return parsed
	}

	return cfg
}
