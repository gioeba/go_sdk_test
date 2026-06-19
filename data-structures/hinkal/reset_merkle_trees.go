package hinkal

import (
	"context"
	"errors"
	"sync"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/crypto"
	"github.com/gioeba/go_sdk_test/internal/data-structures/eventservice"
	"github.com/gioeba/go_sdk_test/internal/data-structures/snapshot"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
)

var resetMerkleTreesMutexByChain sync.Map

func getResetMerkleTreesMutex(chainID int) *sync.Mutex {
	mutex, _ := resetMerkleTreesMutexByChain.LoadOrStore(chainID, &sync.Mutex{})
	m, ok := mutex.(*sync.Mutex)
	if !ok {
		panic("reset merkle trees mutex registry holds a non-*sync.Mutex value")
	}
	return m
}

func resetMerkleTrees(ctx context.Context, h *Hinkal, chainIDs ...int) error {
	chainsToReset := chainIDs
	if len(chainsToReset) == 0 {
		chainsToReset = h.GetSupportedChains()
	}

	var wg sync.WaitGroup
	errs := make([]error, len(chainsToReset))
	for i, chainID := range chainsToReset {
		wg.Add(1)
		go func(i int, chainID int) {
			defer wg.Done()
			errs[i] = resetMerkleTreeForChain(ctx, h, chainID)
		}(i, chainID)
	}
	wg.Wait()

	return errors.Join(errs...)
}

func resetMerkleTreeForChain(ctx context.Context, h *Hinkal, chainID int) error {
	mutex := getResetMerkleTreesMutex(chainID)
	mutex.Lock()
	defer mutex.Unlock()

	hinkalAddress, err := constants.HinkalAddress(chainID)
	if err != nil {
		return err
	}
	fetcher := snapshot.NewSnapshotFetcherService(chainID, hinkalAddress)

	rpcURL, err := constants.FetchRPCURL(chainID)
	if err != nil {
		return err
	}

	eventsFetchingMutex := utils.GetChainBalanceFetchingMutex(chainID)

	if constants.IsSolanaLike(chainID) {
		emitter := eventservice.NewSolanaBlockchainEventEmitter(chainID, rpcURL, hinkalAddress, 0, false, eventsFetchingMutex, nil)
		commitments := snapshot.NewClientSolanaCommitmentsSnapshotService(emitter, crypto.PoseidonHashFunc, fetcher)
		nullifiers := snapshot.NewClientSolanaNullifierSnapshotService(emitter, fetcher)
		if err := commitments.Svc.Init(ctx); err != nil {
			return err
		}
		if err := nullifiers.Svc.Init(ctx); err != nil {
			return err
		}
		if err := emitter.RetrieveEvents(ctx, emitter.LatestBlockNumber()+1, false); err != nil {
			return err
		}
		h.storeChainState(chainID, commitments, nullifiers)
		return nil
	}

	emitter, err := eventservice.NewEVMEmitter(chainID, rpcURL, hinkalAddress, 0, eventsFetchingMutex)
	if err != nil {
		return err
	}
	commitments := snapshot.NewClientCommitmentsSnapshotService(emitter, crypto.PoseidonHashFunc, fetcher)
	nullifiers := snapshot.NewClientNullifierSnapshotService(emitter, fetcher)
	if err := commitments.Svc.Init(ctx); err != nil {
		return err
	}
	if err := nullifiers.Svc.Init(ctx); err != nil {
		return err
	}
	if err := emitter.RetrieveEvents(ctx, emitter.LatestBlockNumber()+1, false); err != nil {
		return err
	}
	h.storeChainState(chainID, commitments, nullifiers)
	return nil
}
