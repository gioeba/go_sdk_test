package providers

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	tronclient "github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

var errTronNoChainID = errors.New("TronProviderAdapter: Chain Id Not Set")

type TronProviderAdapter struct {
	chainID   *int
	evmClient *ethclient.Client
	tronWeb   *SignableTronClient
}

func NewTronProviderAdapter(chainID int) *TronProviderAdapter {
	id := chainID
	return &TronProviderAdapter{chainID: &id}
}

func (a *TronProviderAdapter) InitConnector(ctx context.Context, connector types.TronSigner) error {
	if a.chainID == nil {
		return errTronNoChainID
	}
	address, err := connector.GetAddress(ctx)
	if err != nil {
		return fmt.Errorf("get address: %w", err)
	}
	grpcURL, err := constants.TronGrpcURLFor(*a.chainID)
	if err != nil {
		return err
	}
	evmRPCURL, err := constants.FetchRPCURL(*a.chainID)
	if err != nil {
		return err
	}
	grpcClient := tronclient.NewGrpcClientWithTimeout(grpcURL, 30*time.Second)
	if err := grpcClient.Start(tronclient.GRPCInsecure()); err != nil {
		return fmt.Errorf("start tron gRPC client: %w", err)
	}
	evmClient, err := ethclient.Dial(evmRPCURL)
	if err != nil {
		grpcClient.Stop()
		return fmt.Errorf("dial tron EVM compat RPC: %w", err)
	}
	a.evmClient = evmClient
	a.tronWeb = newSignableTronClient(grpcClient, connector, address)
	return nil
}

func (a *TronProviderAdapter) Init(chainID *int) error {
	a.chainID = chainID
	return nil
}

func (a *TronProviderAdapter) ConnectToConnector() (int, error) {
	if a.chainID == nil {
		return 0, nil
	}
	return *a.chainID, nil
}

func (a *TronProviderAdapter) DisconnectFromConnector() error {
	return nil
}

func (a *TronProviderAdapter) ConnectAndPatchProvider(_ context.Context) (int, error) {
	if a.chainID == nil {
		return 0, errTronNoChainID
	}
	return *a.chainID, nil
}

func (a *TronProviderAdapter) GetChainID() *int {
	return a.chainID
}

func (a *TronProviderAdapter) WaitForTransaction(ctx context.Context, _ int, txHash string, confirmations uint64) (bool, error) {
	if a.tronWeb == nil {
		return false, errors.New("IllegalState: tronWeb not initialized, call InitConnector first")
	}
	return waitForTronTransaction(ctx, a.tronWeb.GrpcClient(), txHash, confirmations)
}

func waitForTronTransaction(ctx context.Context, grpc *tronclient.GrpcClient, txHash string, confirmations uint64) (bool, error) {
	for {
		info, err := grpc.GetTransactionInfoByIDCtx(ctx, txHash)
		if err == nil {
			if info.GetResult() == core.TransactionInfo_FAILED {
				message := string(info.GetResMessage())
				if message == "" {
					message = info.GetResult().String()
				}
				return false, fmt.Errorf("tron transaction failed: %s", message)
			}
			if confirmations <= 1 {
				return true, nil
			}
			targetBlock := info.GetBlockNumber() + int64(confirmations) - 1
			for {
				block, err := grpc.GetNowBlockCtx(ctx)
				if err == nil && block.GetBlockHeader().GetRawData().GetNumber() >= targetBlock {
					return true, nil
				}
				select {
				case <-ctx.Done():
					return false, fmt.Errorf("timeout waiting for Tron confirmations on %s: %w", txHash, ctx.Err())
				case <-time.After(2 * time.Second):
				}
			}
		}

		select {
		case <-ctx.Done():
			return false, fmt.Errorf("timeout waiting for Tron transaction %s: %w", txHash, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

func (a *TronProviderAdapter) SignMessage(ctx context.Context, message string) (string, error) {
	if a.tronWeb == nil {
		return "", errors.New("IllegalState: no signer, call InitConnector first")
	}
	sig, err := a.tronWeb.signer.SignMessage(ctx, message)
	if err != nil {
		return "", err
	}
	return hexSignature(sig)
}

func (a *TronProviderAdapter) GetAddress(_ context.Context) (string, error) {
	if a.tronWeb == nil {
		return "", errors.New("IllegalState")
	}
	return a.tronWeb.GetAddress(), nil
}

func (a *TronProviderAdapter) SwitchNetwork(network types.EthereumNetwork) error {
	id := network.ChainID
	a.chainID = &id
	return nil
}

func (a *TronProviderAdapter) SignTypedData(_ context.Context, _ []byte) (string, error) {
	return "", errors.New("typed data signing not supported on Tron")
}

func (a *TronProviderAdapter) OnAccountChanged() error {
	return nil
}

func (a *TronProviderAdapter) OnChainChanged(_ *int) error {
	return nil
}

func (a *TronProviderAdapter) Release() {
	if a.tronWeb != nil {
		a.tronWeb.GrpcClient().Stop()
	}
	if a.evmClient != nil {
		a.evmClient.Close()
	}
	a.tronWeb = nil
}

func (a *TronProviderAdapter) GetTransactOpts(_ context.Context) (*bind.TransactOpts, error) {
	return nil, errors.New("not implemented for TronProviderAdapter")
}

func (a *TronProviderAdapter) GetFetchClient(_ int) (*ethclient.Client, error) {
	if a.evmClient == nil {
		return nil, errors.New("IllegalState: evmClient not initialized, call InitConnector first")
	}
	return a.evmClient, nil
}

func (a *TronProviderAdapter) SendTransaction(_ context.Context, _ types.TransactionRequest) (string, error) {
	return "", errors.New("not implemented for TronProviderAdapter")
}

func (a *TronProviderAdapter) GetGasPrice(_ context.Context, _ int) (*big.Int, error) {
	return nil, errors.New("not implemented for TronProviderAdapter")
}

func (a *TronProviderAdapter) IsPermitterAvailable() bool {
	return false
}

func (a *TronProviderAdapter) GetTronWeb() *SignableTronClient {
	return a.tronWeb
}
