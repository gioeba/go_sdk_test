package tests

import (
	"context"
	"math/big"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/crypto"
	"github.com/gioeba/go_sdk_test/internal/data-structures/eventservice"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal"
	"github.com/gioeba/go_sdk_test/internal/data-structures/snapshot"
	"github.com/gioeba/go_sdk_test/internal/functions/balance"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

func liveSignature(t *testing.T) string {
	t.Helper()
	sig := os.Getenv("HINKAL_SIGNATURE")
	if sig == "" {
		t.Skip("set HINKAL_SIGNATURE to your Hinkal login signature")
	}
	return sig
}

func liveChainID(t *testing.T) int {
	t.Helper()
	chainID := constants.ChainIDs.EthMainnet
	if v := os.Getenv("HINKAL_CHAIN_ID"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("bad HINKAL_CHAIN_ID: %v", err)
		}
		chainID = parsed
	}
	return chainID
}

func loadChainState(t *testing.T, ctx context.Context, chainID int) (encOutputs []*types.EncryptedOutputWithSign, nullifierSet map[string]struct{}, latestBlock uint64) {
	t.Helper()
	raw, err := api.FetchSnapshots(ctx, chainID)
	if err != nil {
		t.Fatalf("fetch snapshots: %v", err)
	}
	fetcher := snapshot.NewSnapshotFetcherService(chainID, raw.Commitments.HinkalAddress)

	if constants.IsSolanaLike(chainID) {
		emitter := eventservice.NewSolanaBlockchainEventEmitter(chainID, solanaRPC(), raw.Commitments.HinkalAddress, 0, false, nil, nil)
		commitments := snapshot.NewClientSolanaCommitmentsSnapshotService(emitter, crypto.PoseidonHashFunc, fetcher)
		nullifiers := snapshot.NewClientSolanaNullifierSnapshotService(emitter, fetcher)
		if err := commitments.Svc.Init(ctx); err != nil {
			t.Fatalf("init commitments: %v", err)
		}
		if err := nullifiers.Svc.Init(ctx); err != nil {
			t.Fatalf("init nullifiers: %v", err)
		}
		if err := emitter.RetrieveEvents(ctx, emitter.LatestBlockNumber()+1, false); err != nil {
			t.Fatalf("gap-fill: %v", err)
		}
		return commitments.EncryptedOutputs(), nullifiers.Nullifiers(), raw.Commitments.LatestBlockNumber
	}

	rpcURL, rpcErr := constants.FetchRPCURL(chainID)
	if rpcErr != nil {
		t.Skipf("no RPC for chain %d: %v", chainID, rpcErr)
	}
	emitter, err := eventservice.NewEVMEmitter(chainID, rpcURL, raw.Commitments.HinkalAddress, 0, nil)
	if err != nil {
		t.Fatalf("evm emitter: %v", err)
	}
	commitments := snapshot.NewClientCommitmentsSnapshotService(emitter, crypto.PoseidonHashFunc, fetcher)
	nullifiers := snapshot.NewClientNullifierSnapshotService(emitter, fetcher)
	if err := commitments.Svc.Init(ctx); err != nil {
		t.Fatalf("init commitments: %v", err)
	}
	if err := nullifiers.Svc.Init(ctx); err != nil {
		t.Fatalf("init nullifiers: %v", err)
	}
	if err := emitter.RetrieveEvents(ctx, emitter.LatestBlockNumber()+1, false); err != nil {
		t.Fatalf("gap-fill: %v", err)
	}
	return commitments.EncryptedOutputs(), nullifiers.Nullifiers(), raw.Commitments.LatestBlockNumber
}

func sumBalancesPerToken(utxos []*utxo.Utxo, chainID int) map[string]*big.Int {
	balances := map[string]*big.Int{}
	for _, u := range utxos {
		token := balanceKey(u, chainID)
		if balances[token] == nil {
			balances[token] = new(big.Int)
		}
		balances[token].Add(balances[token], u.Amount)
	}
	return balances
}

func balanceKey(u *utxo.Utxo, chainID int) string {
	if constants.IsSolanaLike(chainID) || constants.IsTronLike(chainID) {
		addr, _ := u.GetTokenAddress(chainID)
		return addr
	}
	return common.HexToAddress(u.Erc20TokenAddress).Hex()
}

// HINKAL_LIVE=1 HINKAL_SIGNATURE=0x... [HINKAL_CHAIN_ID=1] go test ./tests/... -run TestRemoteBalance_Live -v
func TestRemoteBalance_Live(t *testing.T) {
	requireLive(t)
	sig := liveSignature(t)
	chainID := liveChainID(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	h := hinkal.NewHinkal(nil)
	h.InitUserKeysWithSignature(sig)
	if err := h.ResetMerkle(ctx, chainID); err != nil {
		t.Fatalf("reset merkle: %v", err)
	}

	localUtxos, err := balance.GetInputUtxoAndBalance(ctx, balance.InputUtxoParams{
		Hinkal:                h,
		ChainID:               chainID,
		ResetCacheBefore:      true,
		AllowRemoteDecryption: false,
	})
	if err != nil {
		t.Fatalf("local decryption: %v", err)
	}

	remoteUtxos, err := balance.GetInputUtxoAndBalance(ctx, balance.InputUtxoParams{
		Hinkal:                h,
		ChainID:               chainID,
		ResetCacheBefore:      true,
		AllowRemoteDecryption: true,
	})
	if err != nil {
		t.Fatalf("remote decryption: %v", err)
	}

	localBalances := sumBalancesPerToken(localUtxos, chainID)
	remoteBalances := sumBalancesPerToken(remoteUtxos, chainID)

	t.Logf("chain=%d localUtxos=%d remoteUtxos=%d", chainID, len(localUtxos), len(remoteUtxos))
	for token, amt := range localBalances {
		t.Logf("local  %s = %s", token, amt.String())
	}
	for token, amt := range remoteBalances {
		t.Logf("remote %s = %s", token, amt.String())
	}

	if len(localBalances) != len(remoteBalances) {
		t.Errorf("token count mismatch: local=%d remote=%d", len(localBalances), len(remoteBalances))
	}
	for token, localAmt := range localBalances {
		remoteAmt, ok := remoteBalances[token]
		if !ok {
			t.Errorf("token %s present locally but missing remotely", token)
			continue
		}
		if localAmt.Cmp(remoteAmt) != 0 {
			t.Errorf("balance mismatch for %s: local=%s remote=%s", token, localAmt.String(), remoteAmt.String())
		}
	}
}

// HINKAL_LIVE=1 HINKAL_SIGNATURE=0x... [HINKAL_CHAIN_ID=1] go test ./tests/... -run TestHinkalGetTotalBalance_Live -v
func TestHinkalGetTotalBalance_Live(t *testing.T) {
	requireLive(t)
	sig := liveSignature(t)
	chainID := liveChainID(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	h := hinkal.NewHinkal(nil)
	h.InitUserKeysWithSignature(sig)
	if err := h.ResetMerkle(ctx, chainID); err != nil {
		t.Fatalf("reset merkle: %v", err)
	}

	balances, err := h.GetTotalBalance(ctx, chainID, nil, "", false, false)
	if err != nil {
		t.Fatalf("getTotalBalance: %v", err)
	}

	t.Logf("chain=%d tokens=%d", chainID, len(balances))
	for _, b := range balances {
		t.Logf("balance %s = %s", b.Token.Erc20TokenAddress, b.Balance.String())
	}
}
