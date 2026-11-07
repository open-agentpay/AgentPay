package onchain

// Known EVM network and stablecoin constants for AgentPay overseas settlement.
const (
	DefaultOnchainNetwork = "eip155:421614" // Arbitrum Sepolia
	DefaultUSDCAsset      = "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d"
	DefaultUSDCSymbol     = "USDC"
	DefaultUSDCDecimals   = 6
)

// NetworkConfig maps CAIP-2 network identifiers to chain properties.
type NetworkConfig struct {
	NetworkID    string
	ChainID      int64
	USDCContract string
	Name         string
}

// KnownNetworks returns standard supported EVM networks for AgentPay.
var KnownNetworks = map[string]NetworkConfig{
	"eip155:421614": {
		NetworkID:    "eip155:421614",
		ChainID:      421614,
		USDCContract: "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d",
		Name:         "Arbitrum Sepolia",
	},
	"eip155:84532": {
		NetworkID:    "eip155:84532",
		ChainID:      84532,
		USDCContract: "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
		Name:         "Base Sepolia",
	},
}
