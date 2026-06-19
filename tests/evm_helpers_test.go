package tests

import (
	"context"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/gioeba/go_sdk_test/data-structures/hinkal"
	"github.com/gioeba/go_sdk_test/providers"
	"github.com/gioeba/go_sdk_test/signers"
	"github.com/gioeba/go_sdk_test/types"
)

const arcTestnetUSDC = "0x3600000000000000000000000000000000000000"

func livePrivateKey(t *testing.T) string {
	t.Helper()
	pk := os.Getenv("HINKAL_PRIVATE_KEY")
	if pk == "" {
		t.Skip("set HINKAL_PRIVATE_KEY to a funded wallet key to run the deposit test")
	}
	return pk
}

// newLiveEVMHinkal wires a private-key signer + EVM provider adapter into a fresh Hinkal,
// derives the user keys from the protocol login signature, and syncs the merkle tree.
func newLiveEVMHinkal(t *testing.T, ctx context.Context, chainID int) (*hinkal.Hinkal, string) {
	t.Helper()
	signer, err := signers.NewPrivateKeyEVMSigner(livePrivateKey(t))
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	adapter, err := providers.NewEthersProviderAdapter()
	if err != nil {
		t.Fatalf("adapter: %v", err)
	}
	adapter.InitSigner(signer)
	cid := chainID
	if err := adapter.Init(&cid); err != nil {
		t.Fatalf("adapter init: %v", err)
	}

	h := hinkal.NewHinkal(nil)
	if err := h.InitProviderAdapter(ctx, adapter); err != nil {
		t.Fatalf("init provider adapter: %v", err)
	}

	if seed := os.Getenv("HINKAL_SEED_PHRASE"); seed != "" {
		h.InitUserKeysFromSeedPhrases(strings.Fields(seed))
	} else if err := h.InitUserKeys(ctx, types.LoginMessageModeProtocol); err != nil {
		t.Fatalf("init user keys: %v", err)
	}
	if err := h.ResetMerkle(ctx, chainID); err != nil {
		t.Fatalf("reset merkle: %v", err)
	}

	ethAddress, err := h.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		t.Fatalf("eth address: %v", err)
	}
	return h, ethAddress
}

func privateBalanceForToken(t *testing.T, ctx context.Context, h *hinkal.Hinkal, chainID int, ethAddress, tokenAddress string) *big.Int {
	t.Helper()
	if err := h.ResetMerkle(ctx, chainID); err != nil {
		t.Fatalf("reset merkle: %v", err)
	}
	balances, err := h.GetTotalBalance(ctx, chainID, nil, ethAddress, true, false)
	if err != nil {
		t.Fatalf("get total balance: %v", err)
	}
	for _, b := range balances {
		if strings.EqualFold(b.Token.Erc20TokenAddress, tokenAddress) {
			return b.Balance
		}
	}
	return new(big.Int)
}
