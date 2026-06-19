package eventservice

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/contractabi"
)

type noopEmitterDelegate struct{}

func (noopEmitterDelegate) Clear()                                                       {}
func (noopEmitterDelegate) StartUpdateListener(context.Context, *BlockchainEventEmitter) {}

func NewEVMEmitter(chainID int, rpcURL, contractAddress string, initialBlock uint64, eventsFetchingMutex *sync.Mutex) (*BlockchainEventEmitter, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial rpc: %w", err)
	}
	parsedABI, err := contractabi.Hinkal(chainID)
	if err != nil {
		return nil, fmt.Errorf("load abi: %w", err)
	}
	contractData, err := constants.GetContractData(chainID)
	if err != nil {
		return nil, fmt.Errorf("load contract data: %w", err)
	}
	var depositOnChainUtxosAddr *common.Address
	if contractData.DepositOnChainUtxosExternalActionAddress != "" {
		addr := common.HexToAddress(contractData.DepositOnChainUtxosExternalActionAddress)
		depositOnChainUtxosAddr = &addr
	}
	return New(
		chainID,
		client,
		common.HexToAddress(contractAddress),
		parsedABI,
		initialBlock,
		false,
		noopEmitterDelegate{},
		eventsFetchingMutex,
		nil,
		depositOnChainUtxosAddr,
	), nil
}
