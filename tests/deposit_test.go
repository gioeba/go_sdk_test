package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/gioeba/go_sdk_test/constants"
)

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestDeposit_Live -v
func TestDeposit_Live(t *testing.T) {
	requireLive(t)
	chainID := constants.ChainIDs.ArcTestnet
	amount := big.NewInt(100_000) // 0.1 USDC (6 decimals)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, chainID)
	before := privateBalanceForToken(t, ctx, h, chainID, ethAddress, arcTestnetUSDC)

	_, txHash, err := h.Deposit(ctx, chainID, []string{arcTestnetUSDC}, []*big.Int{amount}, true, false)
	if err != nil {
		t.Fatalf("deposit: %v", err)
	}
	t.Logf("deposit tx: %s", txHash)
	if _, err := h.WaitForTransaction(ctx, chainID, txHash, 1); err != nil {
		t.Fatalf("wait for tx: %v", err)
	}
	time.Sleep(10 * time.Second) // let the snapshot server catch up

	after := privateBalanceForToken(t, ctx, h, chainID, ethAddress, arcTestnetUSDC)
	delta := new(big.Int).Sub(after, before)
	t.Logf("private USDC: before=%s after=%s delta=%s", before, after, delta)
	if delta.Cmp(amount) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, amount)
	}
}

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestDepositForOther_Live -v
func TestDepositForOther_Live(t *testing.T) {
	requireLive(t)
	chainID := constants.ChainIDs.ArcTestnet
	amount := big.NewInt(100_000)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, chainID)
	recipientInfo, err := h.GetRecipientInfo() // deposit to our own shielded account to verify
	if err != nil {
		t.Fatalf("recipient info: %v", err)
	}
	before := privateBalanceForToken(t, ctx, h, chainID, ethAddress, arcTestnetUSDC)

	_, txHash, err := h.DepositForOther(ctx, chainID, []string{arcTestnetUSDC}, []*big.Int{amount}, recipientInfo, true, false)
	if err != nil {
		t.Fatalf("deposit for other: %v", err)
	}
	t.Logf("deposit-for-other tx: %s", txHash)
	if _, err := h.WaitForTransaction(ctx, chainID, txHash, 1); err != nil {
		t.Fatalf("wait for tx: %v", err)
	}
	time.Sleep(10 * time.Second)

	after := privateBalanceForToken(t, ctx, h, chainID, ethAddress, arcTestnetUSDC)
	delta := new(big.Int).Sub(after, before)
	t.Logf("private USDC: before=%s after=%s delta=%s", before, after, delta)
	if delta.Cmp(amount) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, amount)
	}
}

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestProoflessDeposit_Live -v
func TestProoflessDeposit_Live(t *testing.T) {
	requireLive(t)
	chainID := constants.ChainIDs.ArcTestnet
	amount := big.NewInt(100_000)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, chainID)
	before := privateBalanceForToken(t, ctx, h, chainID, ethAddress, arcTestnetUSDC)

	_, txHash, err := h.ProoflessDeposit(ctx, chainID, []string{arcTestnetUSDC}, []*big.Int{amount}, nil, false)
	if err != nil {
		t.Fatalf("proofless deposit: %v", err)
	}
	t.Logf("proofless deposit tx: %s", txHash)
	if _, err := h.WaitForTransaction(ctx, chainID, txHash, 1); err != nil {
		t.Fatalf("wait for tx: %v", err)
	}
	time.Sleep(10 * time.Second)

	after := privateBalanceForToken(t, ctx, h, chainID, ethAddress, arcTestnetUSDC)
	delta := new(big.Int).Sub(after, before)
	t.Logf("private USDC: before=%s after=%s delta=%s", before, after, delta)
	if delta.Cmp(amount) != 0 {
		t.Fatalf("private balance delta = %s, want %s", delta, amount)
	}
}
