// mirrors chains.constants.ts
package constants

import (
	"fmt"
	"os"

	"github.com/gioeba/go_sdk_test/types"
)

type chainIDs struct {
	Polygon        int
	ArbMainnet     int
	EthMainnet     int
	Optimism       int
	Base           int
	Localhost      int
	ArcTestnet     int
	SepoliaTestnet int
	SolanaMainnet  int
	SolanaLocalnet int
	TronNile       int
	TronLocalnet   int
	TronMainnet    int
	Tempo          int

	// Bridge-destination-only chains (no Hinkal contracts deployed; reachable only as LiFi bridge targets).
	BNBMainnet int
	Avalanche  int
	Cronos     int
	Monad      int
	Plasma     int
	Ink        int
	HyperEVM   int
}

var ChainIDs = chainIDs{
	Polygon:        137,
	ArbMainnet:     42161,
	EthMainnet:     1,
	Optimism:       10,
	Base:           8453,
	Localhost:      31337,
	ArcTestnet:     5042002,
	SepoliaTestnet: 11155111,
	SolanaMainnet:  501,
	SolanaLocalnet: 102,
	TronNile:       3448148188,
	TronLocalnet:   103,
	TronMainnet:    728126428,
	Tempo:          4217,

	// Bridge-destination-only chains (no Hinkal contracts deployed; reachable only as LiFi bridge targets).
	BNBMainnet: 56,
	Avalanche:  43114,
	Cronos:     25,
	Monad:      143,
	Plasma:     9745,
	Ink:        57073,
	HyperEVM:   999,
}

const SolanaChainIDStr = "4sGjMW1sUnHzSxGspuhpqLDx6wiyjNtZ"

var LocalhostNetwork = ChainIDs.EthMainnet

func IsLocalNetwork(chainID int) bool {
	return chainID == ChainIDs.Localhost
}

func GetNonLocalhostChainID(chainID int) int {
	if chainID != 0 {
		if IsLocalNetwork(chainID) {
			return LocalhostNetwork
		}
		return chainID
	}
	return LocalhostNetwork
}

const AlchemyTestKey = ""

// AlchemyAPIKey returns the Alchemy API key, preferring the ALCHEMY_API_KEY environment variable
// and falling back to the public test key.
func AlchemyAPIKey() string {
	if key := os.Getenv("ALCHEMY_API_KEY"); key != "" {
		return key
	}
	return AlchemyTestKey
}

// heliusURL returns the Helius Solana RPC URL built from the HELIUS_API_KEY environment
// variable. Returns an empty string when the key is unset so no secret is embedded in source.
func heliusURL() string {
	if key := os.Getenv("HELIUS_API_KEY"); key != "" {
		return "https://mainnet.helius-rpc.com/?api-key=" + key
	}
	return ""
}

func alchemyURL(subdomain string) string {
	return fmt.Sprintf("https://%s.g.alchemy.com/v2/%s", subdomain, AlchemyAPIKey())
}

func alchemyWS(subdomain string) string {
	return fmt.Sprintf("wss://%s.g.alchemy.com/v2/%s", subdomain, AlchemyAPIKey())
}

var EthereumNetworkRegistry = map[int]types.EthereumNetwork{
	ChainIDs.EthMainnet: {
		Name:        "Ethereum",
		ChainID:     ChainIDs.EthMainnet,
		RPCURL:      "https://rpc.ankr.com/eth",
		FetchRPCURL: alchemyURL("eth-mainnet"),
		WsRPCURL:    alchemyWS("eth-mainnet"),
		Supported:   true,
		Priority:    1,
		MaxPageSize: 900000,
	},
	ChainIDs.ArbMainnet: {
		Name:        "Arbitrum",
		ChainID:     ChainIDs.ArbMainnet,
		RPCURL:      "https://endpoints.omniatech.io/v1/arbitrum/one/public",
		FetchRPCURL: alchemyURL("arb-mainnet"),
		WsRPCURL:    alchemyWS("arb-mainnet"),
		Supported:   true,
		Priority:    2,
		MaxPageSize: 500000,
	},
	ChainIDs.Optimism: {
		Name:        "Optimism",
		ChainID:     ChainIDs.Optimism,
		RPCURL:      "https://optimism-mainnet.infura.io/v3/c26b99456bb6464bb498926ff5162903",
		FetchRPCURL: alchemyURL("opt-mainnet"),
		WsRPCURL:    alchemyWS("opt-mainnet"),
		Supported:   true,
		Priority:    3,
		MaxPageSize: 900000,
	},
	ChainIDs.Polygon: {
		Name:        "Polygon",
		ChainID:     ChainIDs.Polygon,
		RPCURL:      "https://polygon-rpc.com",
		FetchRPCURL: alchemyURL("polygon-mainnet"),
		WsRPCURL:    alchemyWS("polygon-mainnet"),
		Supported:   true,
		Priority:    4,
		MaxPageSize: 900000,
	},
	ChainIDs.Base: {
		Name:        "Base",
		ChainID:     ChainIDs.Base,
		RPCURL:      "https://mainnet.base.org/",
		FetchRPCURL: alchemyURL("base-mainnet"),
		WsRPCURL:    alchemyWS("base-mainnet"),
		Supported:   true,
		Priority:    7,
		MaxPageSize: 500000,
	},
	ChainIDs.ArcTestnet: {
		Name:        "Arc Testnet",
		ChainID:     ChainIDs.ArcTestnet,
		RPCURL:      alchemyURL("arc-testnet"),
		FetchRPCURL: alchemyURL("arc-testnet"),
		WsRPCURL:    alchemyWS("arc-testnet"),
		Supported:   true,
		Priority:    8,
		MaxPageSize: 9999,
	},
	ChainIDs.SolanaMainnet: {
		Name:        "Solana",
		ChainID:     ChainIDs.SolanaMainnet,
		RPCURL:      "https://api.mainnet-beta.solana.com",
		FetchRPCURL: heliusURL(),
		Supported:   true,
		Priority:    8,
	},
	ChainIDs.SolanaLocalnet: {
		Name:        "Solana Localnet",
		ChainID:     ChainIDs.SolanaLocalnet,
		RPCURL:      "http://127.0.0.1:8899",
		FetchRPCURL: "http://127.0.0.1:8899",
		Supported:   true,
		Priority:    9,
	},
	ChainIDs.TronNile: {
		Name:        "Tron Nile",
		ChainID:     ChainIDs.TronNile,
		RPCURL:      alchemyURL("tron-testnet"),
		FetchRPCURL: alchemyURL("tron-testnet"),
		Supported:   true,
		Priority:    9,
		MaxPageSize: 500000,
	},
	ChainIDs.TronMainnet: {
		Name:        "Tron",
		ChainID:     ChainIDs.TronMainnet,
		RPCURL:      alchemyURL("tron-mainnet"),
		FetchRPCURL: alchemyURL("tron-mainnet"),
		Supported:   true,
		Priority:    10,
		MaxPageSize: 500000,
	},
	ChainIDs.SepoliaTestnet: {
		Name:        "Sepolia Testnet",
		ChainID:     ChainIDs.SepoliaTestnet,
		RPCURL:      alchemyURL("eth-sepolia"),
		FetchRPCURL: alchemyURL("eth-sepolia"),
		Supported:   true,
		Priority:    11,
		MaxPageSize: 900000,
	},
	ChainIDs.Tempo: {
		Name:        "Tempo",
		ChainID:     ChainIDs.Tempo,
		RPCURL:      "https://rpc.tempo.xyz",
		FetchRPCURL: "https://rpc.tempo.xyz",
		Supported:   true,
		Priority:    12,
		MaxPageSize: 9999,
	},

	// Bridge-destination-only chains: no Hinkal contracts, only valid as LiFi bridge targets in pay/dashboard.
	ChainIDs.BNBMainnet: {
		Name:        "BNB Chain",
		ChainID:     ChainIDs.BNBMainnet,
		RPCURL:      "https://bsc-dataseed.binance.org",
		FetchRPCURL: alchemyURL("bnb-mainnet"),
		WsRPCURL:    alchemyWS("bnb-mainnet"),
		Supported:   true,
		Priority:    13,
	},
	ChainIDs.Avalanche: {
		Name:        "Avalanche",
		ChainID:     ChainIDs.Avalanche,
		RPCURL:      "https://api.avax.network/ext/bc/C/rpc",
		FetchRPCURL: "https://api.avax.network/ext/bc/C/rpc",
		Supported:   false,
		Priority:    14,
	},
	ChainIDs.Cronos: {
		Name:        "Cronos",
		ChainID:     ChainIDs.Cronos,
		RPCURL:      "https://evm.cronos.org",
		FetchRPCURL: "https://evm.cronos.org",
		Supported:   false,
		Priority:    15,
	},
	ChainIDs.Monad: {
		Name:        "Monad",
		ChainID:     ChainIDs.Monad,
		RPCURL:      "https://rpc.monad.xyz",
		FetchRPCURL: "https://rpc.monad.xyz",
		Supported:   false,
		Priority:    16,
	},
	ChainIDs.Plasma: {
		Name:        "Plasma",
		ChainID:     ChainIDs.Plasma,
		RPCURL:      "https://rpc.plasma.to",
		FetchRPCURL: "https://rpc.plasma.to",
		Supported:   false,
		Priority:    17,
	},
	ChainIDs.Ink: {
		Name:        "Ink",
		ChainID:     ChainIDs.Ink,
		RPCURL:      "https://rpc-gel.inkonchain.com",
		FetchRPCURL: "https://rpc-gel.inkonchain.com",
		Supported:   false,
		Priority:    18,
	},
	ChainIDs.HyperEVM: {
		Name:        "HyperEVM",
		ChainID:     ChainIDs.HyperEVM,
		RPCURL:      "https://rpc.hyperliquid.xyz/evm",
		FetchRPCURL: "https://rpc.hyperliquid.xyz/evm",
		Supported:   false,
		Priority:    19,
	},
}

var TronChainIDs = []int{ChainIDs.TronNile, ChainIDs.TronMainnet, ChainIDs.TronLocalnet}

var SolanaChainIDs = []int{ChainIDs.SolanaMainnet, ChainIDs.SolanaLocalnet}

var HinkalSupportedChains = []int{
	ChainIDs.EthMainnet,
	ChainIDs.Optimism,
	ChainIDs.Base,
	ChainIDs.Polygon,
	ChainIDs.ArbMainnet,
	ChainIDs.ArcTestnet,
	ChainIDs.SolanaMainnet,
	ChainIDs.SepoliaTestnet,
	ChainIDs.TronMainnet,
	ChainIDs.TronNile,
	ChainIDs.Tempo,
}

var EVMChainIDs = func() []int {
	var ids []int
	for _, id := range HinkalSupportedChains {
		if !IsSolanaLike(id) && !IsTronLike(id) {
			ids = append(ids, id)
		}
	}
	return ids
}()

var CurrentSolanaChainID = ChainIDs.SolanaMainnet

func CurrentTronChainID() int {
	if Mode != DeploymentModeProduction {
		return ChainIDs.TronNile
	}
	return ChainIDs.TronMainnet
}

var WalletSupportedChains = func() []int {
	var ids []int
	for _, id := range HinkalSupportedChains {
		if !IsTronLike(id) && !IsSepoliaTestnet(id) && !IsTempo(id) {
			ids = append(ids, id)
		}
	}
	return ids
}()

// Chains that can act as a bridge source (Hinkal contracts deployed, user can hold a balance).
var BridgeSupportedChains = func() []int {
	var ids []int
	for _, id := range HinkalSupportedChains {
		if id != ChainIDs.ArcTestnet && !IsSepoliaTestnet(id) && !(IsTronLike(id) && id != ChainIDs.TronMainnet) {
			ids = append(ids, id)
		}
	}
	return ids
}()

// Chains we can bridge to via LiFi but where Hinkal has no contracts deployed.
// They are valid bridge destinations in pay/dashboard only, never a source/wallet/balance chain.
var BridgeDestinationOnlyChains = []int{
	ChainIDs.BNBMainnet,
	ChainIDs.Avalanche,
	ChainIDs.Cronos,
	ChainIDs.Monad,
	ChainIDs.Plasma,
	ChainIDs.Ink,
	ChainIDs.HyperEVM,
}

// Full set of chains selectable as a bridge destination (source-capable chains + destination-only chains).
var BridgeDestinationChains = append(copyChainIDs(BridgeSupportedChains), BridgeDestinationOnlyChains...)

func IsHinkalSupportedChain(chainID int) bool {
	return contains(HinkalSupportedChains, chainID)
}

func IsBridgeSupportedChain(chainID int) bool {
	return contains(BridgeSupportedChains, chainID)
}

func IsBridgeDestinationOnlyChain(chainID int) bool {
	return contains(BridgeDestinationOnlyChains, chainID)
}

func IsBridgeDestinationChain(chainID int) bool {
	return contains(BridgeDestinationChains, chainID)
}

func GetBridgeDestinationChains(sourceChainID ...int) []int {
	if len(sourceChainID) > 0 && UsesNearIntentsBridge(sourceChainID[0]) {
		var ids []int
		for _, chainID := range BridgeDestinationChains {
			if IsNearBridgeSupportedChain(chainID) {
				ids = append(ids, chainID)
			}
		}
		return ids
	}
	return copyChainIDs(BridgeDestinationChains)
}

var SaveDepths = map[int]uint64{
	ChainIDs.EthMainnet:     1000,
	ChainIDs.BNBMainnet:     1000,
	ChainIDs.Polygon:        4000,
	ChainIDs.ArbMainnet:     8000,
	ChainIDs.Optimism:       6000,
	ChainIDs.Base:           6000,
	ChainIDs.ArcTestnet:     8000,
	ChainIDs.SepoliaTestnet: 1000,
	ChainIDs.Tempo:          4000,
	ChainIDs.Localhost:      1,
	ChainIDs.SolanaMainnet:  1000,
	ChainIDs.SolanaLocalnet: 100,
	ChainIDs.TronNile:       1000,
	ChainIDs.TronMainnet:    1000,
	ChainIDs.TronLocalnet:   100,
}

var BlockReorgDepths = map[int]uint64{
	ChainIDs.EthMainnet:     300,
	ChainIDs.BNBMainnet:     1000,
	ChainIDs.Polygon:        1000,
	ChainIDs.ArbMainnet:     1000,
	ChainIDs.Optimism:       1000,
	ChainIDs.Base:           1000,
	ChainIDs.ArcTestnet:     30,
	ChainIDs.SepoliaTestnet: 300,
	ChainIDs.Tempo:          1000,
	ChainIDs.TronNile:       19,
	ChainIDs.TronMainnet:    19,
	ChainIDs.TronLocalnet:   1,
	ChainIDs.Localhost:      1,
}

func IsOptimismLike(chainID int) bool {
	return chainID == ChainIDs.Optimism || chainID == ChainIDs.Base
}

func IsSolanaLike(chainID int) bool {
	return contains(SolanaChainIDs, chainID)
}

func IsTronLike(chainID int) bool {
	return contains(TronChainIDs, chainID)
}

func IsEvmChain(chainID int) bool {
	return contains(EVMChainIDs, chainID)
}

func UsesNearIntentsBridge(chainID int) bool {
	return IsSolanaLike(chainID) || IsTronLike(chainID)
}

func IsSepoliaTestnet(chainID int) bool {
	return chainID == ChainIDs.SepoliaTestnet
}

func IsTempo(chainID int) bool {
	return chainID == ChainIDs.Tempo
}

// TronGrpcURLFor returns the native gRPC endpoint for a Tron chain.
func TronGrpcURLFor(chainID int) (string, error) {
	urls := map[int]string{
		ChainIDs.TronMainnet: "grpc.trongrid.io:50051",
		ChainIDs.TronNile:    "grpc.nile.trongrid.io:50051",
	}
	url, ok := urls[chainID]
	if !ok {
		return "", fmt.Errorf("no Tron gRPC URL configured for chain %d", chainID)
	}
	return url, nil
}

func GetSaveDepth(chainID int) (uint64, error) {
	if d, ok := SaveDepths[chainID]; ok {
		return d, nil
	}
	return 0, fmt.Errorf("no save depth configured for chain %d", chainID)
}

func GetReorgDepth(chainID int) (uint64, error) {
	if d, ok := BlockReorgDepths[chainID]; ok {
		return d, nil
	}
	return 0, fmt.Errorf("no reorg depth configured for chain %d", chainID)
}

func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func copyChainIDs(ids []int) []int {
	return append([]int(nil), ids...)
}
