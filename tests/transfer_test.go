package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/functions/fees"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/types"
)

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestTransfer_Live -v
func TestTransfer_Live(t *testing.T) {
	requireLive(t)
	chainID := constants.ChainIDs.ArcTestnet
	transferAmount := big.NewInt(50_000) // 0.05 USDC (6 decimals)

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, chainID)

	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		arcTestnetUSDC,
		[]string{arcTestnetUSDC},
		types.ExternalActionTransact,
		nil,
		big.NewInt(constants.HinkalPrivateSendVariableRate),
		nil,
	)
	if err != nil {
		t.Fatalf("fee structure: %v", err)
	}
	totalRelayFee := fees.CalculateTotalFee(transferAmount, feeStructure)
	depositAmount := new(big.Int).Add(transferAmount, totalRelayFee)

	_, depositTxHash, err := h.Deposit(ctx, chainID, []string{arcTestnetUSDC}, []*big.Int{depositAmount}, true, false)
	if err != nil {
		t.Fatalf("deposit before transfer: %v", err)
	}
	t.Logf("deposit tx: %s (amount=%s)", depositTxHash, depositAmount)
	if _, err := h.WaitForTransaction(ctx, chainID, depositTxHash, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}
	time.Sleep(10 * time.Second)

	recipientInfo, err := h.GetRecipientInfo()
	if err != nil {
		t.Fatalf("recipient info: %v", err)
	}
	privateBefore := privateBalanceForToken(t, ctx, h, chainID, ethAddress, arcTestnetUSDC)
	transferChange := new(big.Int).Neg(transferAmount)
	transferTxHash, err := h.Transfer(ctx, chainID, []string{arcTestnetUSDC}, []*big.Int{transferChange}, recipientInfo, arcTestnetUSDC, &feeStructure)
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	t.Logf("transfer tx: %s (amount=%s fee=%s)", transferTxHash, transferAmount, totalRelayFee)
	if _, err := h.WaitForTransaction(ctx, chainID, transferTxHash, 1); err != nil {
		t.Fatalf("wait for transfer tx: %v", err)
	}
	time.Sleep(10 * time.Second)

	privateAfter := privateBalanceForToken(t, ctx, h, chainID, ethAddress, arcTestnetUSDC)
	delta := new(big.Int).Sub(privateAfter, privateBefore)
	expected := new(big.Int).Neg(totalRelayFee)
	t.Logf("private USDC transfer: before=%s after=%s delta=%s want=%s", privateBefore, privateAfter, delta, expected)
	if delta.Cmp(expected) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, expected)
	}
}
