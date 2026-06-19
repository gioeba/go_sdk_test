package constants

import "fmt"

type UniswapV3Addresses struct {
	QuoterV2Address         string
	UniswapV3FactoryAddress string
}

var uniswapV3ByChain = map[int]UniswapV3Addresses{
	ChainIDs.EthMainnet:     {QuoterV2Address: "0x61fFE014bA17989E743c5F6cB21bF9697530B21e", UniswapV3FactoryAddress: "0x1F98431c8aD98523631AE4a59f267346ea31F984"},
	ChainIDs.ArbMainnet:     {QuoterV2Address: "0x61fFE014bA17989E743c5F6cB21bF9697530B21e", UniswapV3FactoryAddress: "0x1F98431c8aD98523631AE4a59f267346ea31F984"},
	ChainIDs.Optimism:       {QuoterV2Address: "0x61fFE014bA17989E743c5F6cB21bF9697530B21e", UniswapV3FactoryAddress: "0x1F98431c8aD98523631AE4a59f267346ea31F984"},
	ChainIDs.Polygon:        {QuoterV2Address: "0x61fFE014bA17989E743c5F6cB21bF9697530B21e", UniswapV3FactoryAddress: "0x1F98431c8aD98523631AE4a59f267346ea31F984"},
	ChainIDs.Base:           {QuoterV2Address: "0x3d4e44Eb1374240CE5F1B871ab261CD16335B76a", UniswapV3FactoryAddress: "0x33128a8fC17869897dcE68Ed026d694621f6FDfD"},
	ChainIDs.SepoliaTestnet: {QuoterV2Address: "0xEd1f6473345F45b75F8179591dd5bA1888cf2FB3", UniswapV3FactoryAddress: "0x0227628f3F023bb0B980b67D528571c95c6DaC1c"},
	ChainIDs.Tempo:          {QuoterV2Address: "0x53ab5d7a69db158f621b43ee70423da1e1403c2a", UniswapV3FactoryAddress: "0x24a3d4757e330890a8b8978028c9e58e04611fd6"},
}

func GetUniswapV3Addresses(chainID int) (UniswapV3Addresses, error) {
	if chainID == ChainIDs.Localhost {
		chainID = LocalhostNetwork
	}
	addresses, ok := uniswapV3ByChain[chainID]
	if !ok {
		return UniswapV3Addresses{}, fmt.Errorf("no Uniswap V3 addresses configured for chain %d", chainID)
	}
	return addresses, nil
}
