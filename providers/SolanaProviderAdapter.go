package providers

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/gioeba/go_sdk_test/constants"
	errorhandling "github.com/gioeba/go_sdk_test/error-handling"
	"github.com/gioeba/go_sdk_test/types"
)

var errSolanaNoWallet = errors.New("IllegalState: no wallet, call InitConnector first")
var errSolanaNoChainID = errors.New("no Chain Id In Provider Adapter")

type SolanaProviderAdapter struct {
	chainID         *int
	client          *rpc.Client
	wallet          types.SolanaSigner
	ethereumAddress string
}

func NewSolanaProviderAdapter(chainID int, ethereumAddress string) (*SolanaProviderAdapter, error) {
	rpcURL, err := constants.FetchRPCURL(chainID)
	if err != nil {
		return nil, err
	}
	id := chainID
	return &SolanaProviderAdapter{
		chainID:         &id,
		client:          rpc.New(rpcURL),
		ethereumAddress: ethereumAddress,
	}, nil
}

func (a *SolanaProviderAdapter) InitConnector(wallet types.SolanaSigner) {
	a.wallet = wallet
}

func (a *SolanaProviderAdapter) Init(chainID *int) error {
	if chainID != nil {
		a.chainID = chainID
	}
	return nil
}

func (a *SolanaProviderAdapter) ConnectToConnector() (int, error) {
	if a.chainID == nil {
		return 0, nil
	}
	return *a.chainID, nil
}

func (a *SolanaProviderAdapter) DisconnectFromConnector() error {
	return nil
}

func (a *SolanaProviderAdapter) ConnectAndPatchProvider(_ context.Context) (int, error) {
	if a.chainID == nil {
		return 0, errSolanaNoChainID
	}
	return *a.chainID, nil
}

func (a *SolanaProviderAdapter) GetChainID() *int {
	return a.chainID
}

func (a *SolanaProviderAdapter) WaitForTransaction(ctx context.Context, _ int, txHash string, _ uint64) (bool, error) {
	sig, err := solana.SignatureFromBase58(txHash)
	if err != nil {
		return false, fmt.Errorf("parse solana signature %s: %w", txHash, err)
	}

	// Solana transactions expire after ~150 slots (~60s). Apply a hard deadline
	// if the caller did not already set one so we never loop forever.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
	}

	for {
		statuses, err := a.client.GetSignatureStatuses(ctx, false, sig)
		if err != nil {
			return false, fmt.Errorf("get signature status %s: %w", txHash, err)
		}
		if len(statuses.Value) > 0 && statuses.Value[0] != nil {
			status := statuses.Value[0]
			if status.Err != nil {
				return false, errorhandling.ErrTransactionNotConfirmed
			}
			if status.ConfirmationStatus == rpc.ConfirmationStatusConfirmed ||
				status.ConfirmationStatus == rpc.ConfirmationStatusFinalized {
				return true, nil
			}
		}
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("timeout waiting for confirmation of %s: %w", txHash, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

func (a *SolanaProviderAdapter) SignMessage(ctx context.Context, message string) (string, error) {
	if a.wallet == nil {
		return "", errSolanaNoWallet
	}
	sig, err := a.wallet.SignMessage(ctx, []byte(message))
	if err != nil {
		return "", err
	}
	return hexSignature(sig)
}

func (a *SolanaProviderAdapter) GetAddress(ctx context.Context) (string, error) {
	if a.ethereumAddress != "" {
		return a.ethereumAddress, nil
	}
	if a.wallet == nil {
		return "", errors.New("IllegalState")
	}
	pubKey, err := a.wallet.GetPublicKey(ctx)
	if err != nil {
		return "", err
	}
	return pubKey.String(), nil
}

func (a *SolanaProviderAdapter) SwitchNetwork(network types.EthereumNetwork) error {
	id := network.ChainID
	a.chainID = &id
	return nil
}

func (a *SolanaProviderAdapter) SignTypedData(_ context.Context, _ []byte) (string, error) {
	return "", errors.New("typed data signing not supported on Solana")
}

func (a *SolanaProviderAdapter) OnAccountChanged() error {
	return nil
}

func (a *SolanaProviderAdapter) OnChainChanged(chainID *int) error {
	return a.Init(chainID)
}

func (a *SolanaProviderAdapter) Release() {
	a.wallet = nil
}

func (a *SolanaProviderAdapter) GetTransactOpts(_ context.Context) (*bind.TransactOpts, error) {
	return nil, errors.New("not implemented for SolanaProviderAdapter")
}

func (a *SolanaProviderAdapter) GetFetchClient(_ int) (*ethclient.Client, error) {
	return nil, errors.New("not implemented for SolanaProviderAdapter")
}

func (a *SolanaProviderAdapter) SendTransaction(_ context.Context, _ types.TransactionRequest) (string, error) {
	return "", errors.New("not implemented for SolanaProviderAdapter")
}

func (a *SolanaProviderAdapter) GetGasPrice(_ context.Context, _ int) (*big.Int, error) {
	return nil, errors.New("not implemented for SolanaProviderAdapter")
}

func (a *SolanaProviderAdapter) IsPermitterAvailable() bool {
	return false
}

// Solana-specific methods

func (a *SolanaProviderAdapter) GetConnection() *rpc.Client {
	return a.client
}

func (a *SolanaProviderAdapter) GetSolanaPublicKey(ctx context.Context) (solana.PublicKey, error) {
	if a.wallet == nil {
		return solana.PublicKey{}, errSolanaNoWallet
	}
	return a.wallet.GetPublicKey(ctx)
}

func (a *SolanaProviderAdapter) GetSolanaProgram(programID solana.PublicKey) (*SolanaProgram, error) {
	if a.wallet == nil {
		return nil, errSolanaNoWallet
	}
	return &SolanaProgram{
		ProgramID: programID,
		Client:    a.client,
		Signer:    a.wallet,
	}, nil
}

func (a *SolanaProviderAdapter) SignTransactionWithoutBroadcast(ctx context.Context, tx *solana.Transaction) ([]byte, string, error) {
	program, err := a.GetSolanaProgram(solana.PublicKey{})
	if err != nil {
		return nil, "", err
	}
	return program.SignOnly(ctx, tx)
}
