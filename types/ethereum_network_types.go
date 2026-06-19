package types

import "math/big"

type ContractType int

const (
	ContractTypeHinkal                ContractType = iota // HinkalContract
	ContractTypeHinkalHelper                              // HinkalHelperContract
	ContractTypeAccessToken                               // AccessTokenContract
	ContractTypeUniswapV3Factory                          // UniswapV3FactoryContract
	ContractTypeUniswapV3Pool                             // UniswapV3PoolContract
	ContractTypeUniswapV3Quoter                           // UniswapV3QuoterContract
	ContractTypeERC20                                     // ERC20Contract
	ContractTypeERC721                                    // ERC721Contract
	ContractTypeERC1155                                   // ERC1155Contract
	ContractTypeWAToken                                   // WATokenContract
	ContractTypeOneInchExternalAction                     // OneInchExternalActionContract
	ContractTypeMerkleTree                                // MerkleTreeContract
	ContractTypeContractWithNonces                        // ContractWithNonces
	ContractTypePermitter                                 // PermitterContract
	ContractTypeUniswapV2Pool                             // UniswapV2PoolContract
	ContractTypeHinkalWrapper                             // HinkalWrapper
	ContractTypeHinkalWrapper2                            // HinkalWrapper2
	ContractTypeDepositOnChainUtxos                       // DepositOnChainUtxos
)

type EthereumNetwork struct {
	Name        string
	ChainID     int
	RPCURL      string
	FetchRPCURL string
	WsRPCURL    string
	Supported   bool
	Priority    int
	MaxPageSize int
}

type TransactionRequest struct {
	To       string
	Data     []byte
	Value    *big.Int
	GasLimit uint64
}
