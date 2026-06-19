package snarkjs

import (
	"context"
	"fmt"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/contractabi"
	"github.com/gioeba/go_sdk_test/internal/data-structures/solana"
)

func FetchOnChainRootHashes(ctx context.Context, chainID int) (*big.Int, error) {
	hinkalAddress, err := constants.HinkalAddress(chainID)
	if err != nil {
		return nil, err
	}
	rpcURL, err := constants.FetchRPCURL(chainID)
	if err != nil {
		return nil, err
	}

	if constants.IsSolanaLike(chainID) {
		originalDeployer, err := constants.OriginalDeployer(chainID)
		if err != nil {
			return nil, err
		}
		client := solana.NewClient(rpcURL)
		return solana.FetchMerkleTreeRootHash(ctx, client, hinkalAddress, originalDeployer)
	}

	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	parsedABI, err := contractabi.Hinkal(chainID)
	if err != nil {
		return nil, err
	}
	data, err := parsedABI.Pack("getRootHash")
	if err != nil {
		return nil, err
	}
	address := common.HexToAddress(hinkalAddress)
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &address, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	results, err := parsedABI.Unpack("getRootHash", out)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("fetchOnChainRootHashes: empty result")
	}
	rootHash, ok := results[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("fetchOnChainRootHashes: unexpected type %T", results[0])
	}
	return rootHash, nil
}
