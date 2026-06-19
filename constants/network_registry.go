package constants

//go:generate node gen/generate-deploy-data.js

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed deploy-data/*.json
var deployDataFS embed.FS

var deployDataFileByChain = map[int]string{
	ChainIDs.EthMainnet:     "deploy-data-ethMainnet.json",
	ChainIDs.ArbMainnet:     "deploy-data-arbMainnet.json",
	ChainIDs.Optimism:       "deploy-data-optimism.json",
	ChainIDs.Polygon:        "deploy-data-polygon.json",
	ChainIDs.Base:           "deploy-data-base.json",
	ChainIDs.ArcTestnet:     "deploy-data-arcTestnet.json",
	ChainIDs.SepoliaTestnet: "deploy-data-sepoliaTestnet.json",
	ChainIDs.Tempo:          "deploy-data-tempo.json",
	ChainIDs.SolanaMainnet:  "deploy-data-solana.json",
	ChainIDs.SolanaLocalnet: "deploy-data-solana.json",
	ChainIDs.TronNile:       "deploy-data-tronNile.json",
	ChainIDs.TronMainnet:    "deploy-data-tronMainnet.json",
	ChainIDs.Localhost:      "deploy-data-localhost.json",
}

var sharedDeployFileByChain = map[int]string{
	ChainIDs.EthMainnet:     "shared-deploy-data-evm.json",
	ChainIDs.ArbMainnet:     "shared-deploy-data-evm.json",
	ChainIDs.Optimism:       "shared-deploy-data-evm.json",
	ChainIDs.Polygon:        "shared-deploy-data-evm.json",
	ChainIDs.Base:           "shared-deploy-data-evm.json",
	ChainIDs.ArcTestnet:     "shared-deploy-data-evm.json",
	ChainIDs.SepoliaTestnet: "shared-deploy-data-evm.json",
	ChainIDs.Tempo:          "shared-deploy-data-evm.json",
	ChainIDs.Localhost:      "shared-deploy-data-evm.json",
	ChainIDs.TronNile:       "shared-deploy-data-tron.json",
	ChainIDs.TronMainnet:    "shared-deploy-data-tron.json",
}

type ContractData struct {
	HinkalAddress                            string
	HinkalHelperAddress                      string
	EmporiumAddress                          string
	HinkalWalletAddress                      string
	HinkalWrapperAddress                     string
	UniswapExternalActionAddress             string
	OdosExternalActionInstanceAddress        string
	OneInchExternalActionInstanceAddress     string
	DepositOnChainUtxosExternalActionAddress string
	OriginalDeployer                         string
}

type deployDataFile struct {
	HinkalAddress                            string          `json:"hinkalAddress"`
	HinkalHelperAddress                      string          `json:"hinkalHelperAddress"`
	EmporiumAddress                          string          `json:"emporiumAddress"`
	HinkalWalletAddress                      string          `json:"hinkalWalletAddress"`
	HinkalWrapperAddress                     string          `json:"hinkalWrapperAddress"`
	UniswapExternalActionAddress             string          `json:"uniswapExternalActionAddress"`
	OdosExternalActionInstanceAddress        string          `json:"odosExternalActionInstanceAddress"`
	OneInchExternalActionInstanceAddress     string          `json:"oneInchExternalActionInstanceAddress"`
	DepositOnChainUtxosExternalActionAddress string          `json:"depositOnChainUtxosExternalActionAddress"`
	OriginalDeployer                         string          `json:"originalDeployer"`
	HinkalABI                                json.RawMessage `json:"hinkalABI"`
}

var (
	deployDataCacheMu sync.Mutex
	deployDataCache   = map[int]*deployDataFile{}
)

func loadDeployData(chainID int) (*deployDataFile, error) {
	deployDataCacheMu.Lock()
	defer deployDataCacheMu.Unlock()

	if cached, ok := deployDataCache[chainID]; ok {
		if cached == nil {
			return nil, fmt.Errorf("no contract data for chain %d", chainID)
		}
		return cached, nil
	}

	file, ok := deployDataFileByChain[chainID]
	if !ok {
		deployDataCache[chainID] = nil
		return nil, fmt.Errorf("no contract data for chain %d", chainID)
	}
	raw, err := deployDataFS.ReadFile("deploy-data/" + file)
	if err != nil {
		deployDataCache[chainID] = nil
		return nil, err
	}
	var parsed deployDataFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	if len(parsed.HinkalABI) == 0 {
		if shared, ok := sharedDeployFileByChain[chainID]; ok {
			sharedRaw, err := deployDataFS.ReadFile("deploy-data/" + shared)
			if err != nil {
				deployDataCache[chainID] = nil
				return nil, err
			}
			var sharedData deployDataFile
			if err := json.Unmarshal(sharedRaw, &sharedData); err != nil {
				return nil, fmt.Errorf("parse %s: %w", shared, err)
			}
			parsed.HinkalABI = sharedData.HinkalABI
		}
	}
	deployDataCache[chainID] = &parsed
	return &parsed, nil
}

func GetContractData(chainID int) (ContractData, error) {
	d, err := loadDeployData(chainID)
	if err != nil {
		return ContractData{}, err
	}
	return ContractData{
		HinkalAddress:                            d.HinkalAddress,
		HinkalHelperAddress:                      d.HinkalHelperAddress,
		EmporiumAddress:                          d.EmporiumAddress,
		HinkalWalletAddress:                      d.HinkalWalletAddress,
		HinkalWrapperAddress:                     d.HinkalWrapperAddress,
		UniswapExternalActionAddress:             d.UniswapExternalActionAddress,
		OdosExternalActionInstanceAddress:        d.OdosExternalActionInstanceAddress,
		OneInchExternalActionInstanceAddress:     d.OneInchExternalActionInstanceAddress,
		DepositOnChainUtxosExternalActionAddress: d.DepositOnChainUtxosExternalActionAddress,
		OriginalDeployer:                         d.OriginalDeployer,
	}, nil
}

func HinkalAddress(chainID int) (string, error) {
	contractData, err := GetContractData(chainID)
	if err != nil {
		return "", err
	}
	return contractData.HinkalAddress, nil
}

func OriginalDeployer(chainID int) (string, error) {
	contractData, err := GetContractData(chainID)
	if err != nil {
		return "", err
	}
	return contractData.OriginalDeployer, nil
}

func HinkalABIJSON(chainID int) ([]byte, error) {
	d, err := loadDeployData(chainID)
	if err != nil {
		return nil, err
	}
	if len(d.HinkalABI) == 0 {
		return nil, fmt.Errorf("no hinkal ABI for chain %d", chainID)
	}
	return d.HinkalABI, nil
}
