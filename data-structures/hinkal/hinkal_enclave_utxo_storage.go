package hinkal

import (
	"context"

	"github.com/gioeba/go_sdk_test/internal/functions/enclave"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

func (h *Hinkal) StoreUtxoInEnclave(
	ctx context.Context,
	senderAddress string,
	recipientEthAddress string,
	u *utxo.Utxo,
	chainID int,
	claimableSignature string,
) error {
	return enclave.StoreUtxoInEnclave(ctx, senderAddress, recipientEthAddress, u, chainID, claimableSignature)
}

func (h *Hinkal) GetUtxosFromEnclave(
	ctx context.Context,
	ethAddress string,
	signature string,
	chainID int,
	isSolanaLedger bool,
	txMessageForSolanaLedger string,
) ([]types.UtxoConstructorParamsWithSenderAddress, error) {
	return enclave.GetUtxosFromEnclave(ctx, ethAddress, signature, chainID, isSolanaLedger, txMessageForSolanaLedger)
}
