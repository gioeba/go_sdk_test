package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/functions/fees"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/types"
)

// HINKAL_LIVE=1 HINKAL_SEED_PHRASE="..." HINKAL_SOLANA_PRIVATE_KEY=base58 go test ./tests/ -run TestSolanaWithdraw_Live -v
func TestSolanaWithdraw_Live(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	chainID := constants.CurrentSolanaChainID
	withdrawAmount := big.NewInt(1_000) // 0.001 USDC (6 decimals)

	h := newLiveSolanaHinkal(t, ctx)

	amountChanges := []*big.Int{new(big.Int).Neg(withdrawAmount)}
	nullifierCount := pretransaction.CalculateSolanaNullifierCount(ctx, h, chainID, []string{solanaMainnetUSDC}, amountChanges)
	solanaParams := &api.SolanaGasEstimateParams{MintTo: solanaMainnetUSDC, NullifierCount: nullifierCount}
	feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, solanaMainnetUSDC, []string{solanaMainnetUSDC}, types.ExternalActionTransact, nil, nil, solanaParams)
	if err != nil {
		t.Fatalf("fee structure: %v", err)
	}
	totalRelayFee := fees.CalculateTotalFee(withdrawAmount, feeStructure)
	depositAmount := new(big.Int).Add(withdrawAmount, totalRelayFee)

	sig, err := h.DepositSolana(ctx, chainID, solanaMainnetUSDC, depositAmount, false)
	if err != nil {
		t.Fatalf("solana deposit before withdraw: %v", err)
	}
	t.Logf("solana deposit sig: %s (amount=%s)", sig, depositAmount)
	if _, err := h.WaitForTransaction(ctx, chainID, sig, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	privateBefore := solanaPrivateBalance(t, ctx, h, solanaMainnetUSDC)
	recipient, err := h.GetSolanaPublicKey(ctx)
	if err != nil {
		t.Fatalf("solana recipient: %v", err)
	}
	withdrawChange := new(big.Int).Neg(withdrawAmount)
	_, withdrawSig, err := h.Withdraw(ctx, chainID, []string{solanaMainnetUSDC}, []*big.Int{withdrawChange}, recipient.String(), false, solanaMainnetUSDC, &feeStructure)
	if err != nil {
		t.Fatalf("solana withdraw: %v", err)
	}
	t.Logf("solana withdraw sig: %s (amount=%s fee=%s)", withdrawSig, withdrawAmount, totalRelayFee)
	if _, err := h.WaitForTransaction(ctx, chainID, withdrawSig, 1); err != nil {
		t.Fatalf("wait for withdraw tx: %v", err)
	}
	time.Sleep(15 * time.Second)

	privateAfter := solanaPrivateBalance(t, ctx, h, solanaMainnetUSDC)
	delta := new(big.Int).Sub(privateAfter, privateBefore)
	expected := new(big.Int).Neg(new(big.Int).Add(withdrawAmount, totalRelayFee))
	t.Logf("solana private USDC withdraw: before=%s after=%s delta=%s want=%s", privateBefore, privateAfter, delta, expected)
	if delta.Cmp(expected) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, expected)
	}
}
