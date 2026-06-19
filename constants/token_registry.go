package constants

//go:generate sh -c "cp ../../shared/erc20-registry/src/constants/*Registry.json token-data/"

import (
	"embed"
	"encoding/json"
	"strings"
	"sync"

	"github.com/gioeba/go_sdk_test/types"
)

//go:embed token-data/*.json
var tokenRegistryFS embed.FS

var tokenRegistryFileByChain = map[int]string{
	ChainIDs.Polygon:        "polygonRegistry.json",
	ChainIDs.ArbMainnet:     "arbMainnetRegistry.json",
	ChainIDs.EthMainnet:     "ethMainnetRegistry.json",
	ChainIDs.Optimism:       "optimismRegistry.json",
	ChainIDs.Base:           "baseRegistry.json",
	ChainIDs.ArcTestnet:     "arcTestnetRegistry.json",
	ChainIDs.SepoliaTestnet: "sepoliaTestnetRegistry.json",
	ChainIDs.Tempo:          "tempoRegistry.json",
	ChainIDs.SolanaMainnet:  "solanaMainnetRegistry.json",
	ChainIDs.SolanaLocalnet: "solanaLocalnetRegistry.json",
	ChainIDs.TronNile:       "tronNileRegistry.json",
	ChainIDs.TronMainnet:    "tronMainnetRegistry.json",
	ChainIDs.BNBMainnet:     "bnbMainnetRegistry.json",
	ChainIDs.Avalanche:      "avalancheRegistry.json",
	ChainIDs.Cronos:         "cronosRegistry.json",
	ChainIDs.Monad:          "monadRegistry.json",
	ChainIDs.Plasma:         "plasmaRegistry.json",
	ChainIDs.Ink:            "inkRegistry.json",
	ChainIDs.HyperEVM:       "hyperEvmRegistry.json",
}

type tokenRegistryFile struct {
	NetworkRegistry []types.ERC20Token `json:"networkRegistry"`
}

var (
	tokenRegistryCacheMu sync.Mutex
	tokenRegistryCache   = map[string][]types.ERC20Token{}
)

func loadTokenRegistry(file string) []types.ERC20Token {
	tokenRegistryCacheMu.Lock()
	defer tokenRegistryCacheMu.Unlock()

	if cached, ok := tokenRegistryCache[file]; ok {
		return cached
	}
	raw, err := tokenRegistryFS.ReadFile("token-data/" + file)
	if err != nil {
		tokenRegistryCache[file] = nil
		return nil
	}
	var parsed tokenRegistryFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		tokenRegistryCache[file] = nil
		return nil
	}
	tokenRegistryCache[file] = parsed.NetworkRegistry
	return parsed.NetworkRegistry
}

func GetERC20Registry(chainID int) []types.ERC20Token {
	file, ok := tokenRegistryFileByChain[chainID]
	if !ok {
		return loadTokenRegistry("localhostRegistry.json")
	}
	return loadTokenRegistry(file)
}

func GetErc20TokensForChain(chainID int) []types.ERC20Token {
	return GetERC20Registry(chainID)
}

func GetERC20Token(address string, chainID int) *types.ERC20Token {
	if address == "" {
		return nil
	}
	tokens := GetErc20TokensForChain(chainID)
	for i := range tokens {
		if strings.EqualFold(tokens[i].Erc20TokenAddress, address) {
			return &tokens[i]
		}
	}
	return nil
}
