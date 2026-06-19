package types

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	solana "github.com/gagliardetto/solana-go"
)

type Signer interface {
	GetAddress(ctx context.Context) (common.Address, error)
	SignMessage(ctx context.Context, message string) ([]byte, error)
	SignTypedData(ctx context.Context, typedDataHash []byte) ([]byte, error)
	SignTransaction(ctx context.Context, tx *ethtypes.Transaction) (*ethtypes.Transaction, error)
	BroadcastTransaction(ctx context.Context, tx *ethtypes.Transaction) (string, error)
}

type TronSigner interface {
	GetAddress(ctx context.Context) (string, error)
	SignMessage(ctx context.Context, message string) ([]byte, error)
	SignTransaction(ctx context.Context, tx *core.Transaction) (*core.Transaction, error)
}

type SolanaSigner interface {
	GetPublicKey(ctx context.Context) (solana.PublicKey, error)
	SignMessage(ctx context.Context, message []byte) ([]byte, error)
	SignTransaction(ctx context.Context, tx *solana.Transaction) (*solana.Transaction, error)
}

// GetContractWithSigner and GetContractWithFetcher are generic functions and cannot
// be interface methods in Go, so they live as package-level functions in providers/.
type IProviderAdapter interface {
	Init(chainID *int) error
	ConnectToConnector() (int, error)
	DisconnectFromConnector() error
	ConnectAndPatchProvider(ctx context.Context) (int, error)
	GetChainID() *int
	WaitForTransaction(ctx context.Context, chainID int, txHash string, confirmations uint64) (bool, error)
	SignMessage(ctx context.Context, message string) (string, error)
	SignTypedData(ctx context.Context, typedDataHash []byte) (string, error)
	GetAddress(ctx context.Context) (string, error)
	SwitchNetwork(network EthereumNetwork) error
	OnAccountChanged() error
	OnChainChanged(chainID *int) error
	Release()
	SendTransaction(ctx context.Context, req TransactionRequest) (string, error)
	GetTransactOpts(ctx context.Context) (*bind.TransactOpts, error)
	GetFetchClient(chainID int) (*ethclient.Client, error)
	IsPermitterAvailable() bool
	GetGasPrice(ctx context.Context, chainID int) (*big.Int, error)
}
