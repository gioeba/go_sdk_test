package tests

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
)

const baseUSDC = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"

func temporarySubAccount(t *testing.T) types.TemporarySubAccount {
	t.Helper()
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate temporary wallet: %v", err)
	}
	return types.TemporarySubAccount{
		Index:      int(time.Now().UnixNano() % 1_000_000_000),
		EthAddress: gethcrypto.PubkeyToAddress(key.PublicKey).Hex(),
		PrivateKey: "0x" + hex.EncodeToString(gethcrypto.FromECDSA(key)),
	}
}

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/... -run TestDepositAndBridge_Live -v
// Defaults to Optimism 0.3 USDC -> Base USDC to the same wallet.
func TestDepositAndBridge_Live(t *testing.T) {
	requireLive(t)
	sourceChainID := constants.ChainIDs.Optimism
	destinationChainID := constants.ChainIDs.Base
	sourceTokenAddress := envOr("HINKAL_BRIDGE_IN_TOKEN", optimismUSDC)
	destinationTokenAddress := envOr("HINKAL_BRIDGE_OUT_TOKEN", baseUSDC)
	amount := envOr("HINKAL_BRIDGE_AMOUNT", "0.3")

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, sourceChainID)
	tokens := web3.ResolveERC20Tokens(sourceChainID, []string{sourceTokenAddress})
	sourceToken := tokens[0]
	destinationToken := web3.ResolveERC20Tokens(destinationChainID, []string{destinationTokenAddress})[0]
	bridgeAmount, err := web3.GetAmountInWei(sourceToken, amount)
	if err != nil {
		t.Fatalf("bridge amount: %v", err)
	}

	tempAccount := temporarySubAccount(t)
	quote, err := web3.GetLifiPrice(ctx, sourceToken, destinationToken, amount, 0.005, tempAccount.EthAddress, ethAddress)
	if err != nil {
		t.Fatalf("lifi quote: %v", err)
	}
	t.Logf("lifi quote: in=%s expected=%s nativeFee=%s temp=%s", bridgeAmount, quote.ExpectedAmount, quote.NativeFee, tempAccount.EthAddress)

	result, err := h.DepositAndBridge(ctx, sourceChainID, sourceTokenAddress, []types.BridgeRecipient{
		{
			RecipientAddress:    ethAddress,
			BridgeAmount:        bridgeAmount,
			Quote:               quote,
			TemporarySubAccount: tempAccount,
		},
	}, nil, nil, true)
	if err != nil {
		t.Fatalf("deposit and bridge: %v", err)
	}
	if result.DepositTxHash == "" {
		t.Fatalf("deposit tx hash is empty")
	}
	if result.ScheduleID == "" {
		t.Fatalf("schedule id is empty")
	}
	t.Logf("deposit and bridge: deposit=%s schedule=%s amount=%s", result.DepositTxHash, result.ScheduleID, bridgeAmount)
}
