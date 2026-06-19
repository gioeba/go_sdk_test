package eventservice

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/internal/data-structures/blockchainevent"
	"github.com/gioeba/go_sdk_test/internal/data-structures/solana"
	"github.com/gioeba/go_sdk_test/types"
)

const (
	solanaReorgDepth         = 32
	solanaBatchLimitMax      = 1000
	solanaBatchLimitMin      = 20
	solanaClientPollInterval = 5 * time.Second
)

type SolanaBlockchainEventEmitter struct {
	chainID     int
	programID   string
	rpc         *solana.Client
	initialSlot uint64
	latestSlot  *uint64
	maxPageSize *uint64
	isServer    bool
	isReady     bool
	inProgress  bool

	eventsFetchingMutex *sync.Mutex
	eventProcessors     []blockchainevent.EventProcessorFunc
	OnEventsProcessed   func(count int)

	eventCategory types.EventCategory
	cancel        context.CancelFunc
}

func NewSolanaBlockchainEventEmitter(chainID int, rpcURL, programID string, initialSlot uint64, isServer bool, eventsFetchingMutex *sync.Mutex, maxPageSize *uint64) *SolanaBlockchainEventEmitter {
	if eventsFetchingMutex == nil {
		eventsFetchingMutex = &sync.Mutex{}
	}
	return &SolanaBlockchainEventEmitter{
		chainID:             chainID,
		programID:           programID,
		rpc:                 solana.NewClient(rpcURL),
		initialSlot:         initialSlot,
		maxPageSize:         maxPageSize,
		isServer:            isServer,
		eventsFetchingMutex: eventsFetchingMutex,
		eventCategory:       types.EventCategoryMain,
	}
}

func (e *SolanaBlockchainEventEmitter) ChainID() int  { return e.chainID }
func (e *SolanaBlockchainEventEmitter) IsReady() bool { return e.isReady }

func (e *SolanaBlockchainEventEmitter) LatestBlockNumber() uint64 {
	if e.latestSlot == nil {
		return e.initialSlot
	}
	return *e.latestSlot
}

func (e *SolanaBlockchainEventEmitter) SyncFromAtMost(slot uint64) {
	if e.latestSlot == nil || slot < *e.latestSlot {
		s := slot
		e.latestSlot = &s
	}
}

func (e *SolanaBlockchainEventEmitter) AdvanceLatestBlockNumber(slot uint64) {
	if e.latestSlot == nil || slot > *e.latestSlot {
		s := slot
		e.latestSlot = &s
	}
}

func (e *SolanaBlockchainEventEmitter) AddEventProcessorFunction(fn blockchainevent.EventProcessorFunc) {
	if e.isReady {
		panic("cannot add processor after Init")
	}
	e.eventProcessors = append(e.eventProcessors, fn)
}

func (e *SolanaBlockchainEventEmitter) IntervalClear() {
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	e.isReady = false
	e.eventProcessors = nil
}

func (e *SolanaBlockchainEventEmitter) Init(ctx context.Context) error {
	if e.isReady {
		return fmt.Errorf("already initialized")
	}
	e.isReady = true
	if err := e.retrieveEvents(ctx, e.LatestBlockNumber()+1, false, true); err != nil {
		return err
	}
	go e.startUpdateListener(ctx)
	return nil
}

func (e *SolanaBlockchainEventEmitter) RetrieveEvents(ctx context.Context, fromSlot uint64, force bool) error {
	return e.retrieveEvents(ctx, fromSlot, force, false)
}

func (e *SolanaBlockchainEventEmitter) retrieveEvents(ctx context.Context, fromSlot uint64, force, initialLoading bool) error {
	e.eventsFetchingMutex.Lock()
	defer e.eventsFetchingMutex.Unlock()
	if e.inProgress && !force {
		return nil
	}
	e.inProgress = true
	defer func() { e.inProgress = false }()

	lastSlot, err := e.getLastSlot(ctx)
	if err != nil {
		return fmt.Errorf("get last slot: %w", err)
	}
	if lastSlot < fromSlot {
		return nil
	}
	events, err := e.getProgramEvents(ctx, fromSlot, lastSlot, initialLoading)
	if err != nil {
		return fmt.Errorf("get program events: %w", err)
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].BlockNumber < events[j].BlockNumber })
	if err := e.processEvents(events, lastSlot); err != nil {
		return err
	}
	e.latestSlot = &lastSlot
	return nil
}

func (e *SolanaBlockchainEventEmitter) getLastSlot(ctx context.Context) (uint64, error) {
	slot, err := e.rpc.GetSlot(ctx)
	if err != nil {
		return 0, err
	}
	if !e.isServer {
		return slot, nil
	}
	safe := slot - solanaReorgDepth + 1
	if cur := e.LatestBlockNumber(); cur > safe {
		return cur, nil
	}
	return safe, nil
}

func (e *SolanaBlockchainEventEmitter) getProgramEvents(ctx context.Context, fromSlot, toSlot uint64, initialLoading bool) ([]*blockchainevent.BlockchainEvent, error) {
	limit := solanaBatchLimitMin
	if initialLoading {
		limit = solanaBatchLimitMax
	}
	sigs, err := e.rpc.GetSignaturesForAddress(ctx, e.programID, limit, "")
	if err != nil {
		return nil, err
	}
	batchLen := len(sigs)
	for batchLen >= limit && sigs[len(sigs)-1].Slot >= fromSlot {
		before := sigs[len(sigs)-1].Signature
		next, err := e.rpc.GetSignaturesForAddress(ctx, e.programID, limit, before)
		if err != nil {
			return nil, err
		}
		if len(next) == 0 {
			break
		}
		sigs = append(sigs, next...)
		batchLen = len(next)
	}

	var events []*blockchainevent.BlockchainEvent
	for _, sig := range sigs {
		if sig.Slot < fromSlot || sig.Slot > toSlot || sig.Err != nil {
			continue
		}
		tx, err := e.rpc.GetTransaction(ctx, sig.Signature)
		if err != nil || tx == nil || tx.Meta == nil || tx.Meta.Err != nil {
			continue
		}
		for _, d := range parseSolanaTransaction(tx.Meta) {
			ev, err := buildSolanaEvent(d, sig.Signature, sig.Slot)
			if err != nil {
				continue
			}
			events = append(events, ev)
		}
	}
	return events, nil
}

func (e *SolanaBlockchainEventEmitter) ProcessExternalEvents(events []*blockchainevent.BlockchainEvent, latestSlot uint64) error {
	e.eventsFetchingMutex.Lock()
	defer e.eventsFetchingMutex.Unlock()
	if err := e.processEvents(events, latestSlot); err != nil {
		return err
	}
	e.AdvanceLatestBlockNumber(latestSlot)
	return nil
}

func (e *SolanaBlockchainEventEmitter) processEvents(events []*blockchainevent.BlockchainEvent, scannedToSlot uint64) error {
	total := 0
	for _, proc := range e.eventProcessors {
		n, err := proc(events, &scannedToSlot)
		if err != nil {
			return err
		}
		total += n
	}
	if e.OnEventsProcessed != nil {
		e.OnEventsProcessed(total)
	}
	return nil
}

func (e *SolanaBlockchainEventEmitter) startUpdateListener(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)
	fetchFrom := e.LatestBlockNumber() + 1
	ticker := time.NewTicker(solanaClientPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resp, err := api.GetSnapshotServerEvents(ctx, e.chainID, e.eventCategory, fetchFrom)
			if err != nil {
				log.Printf("solana snapshot events poll error: %v", err)
				continue
			}
			if len(resp.Events) > 0 {
				events := make([]*blockchainevent.BlockchainEvent, 0, len(resp.Events))
				for _, serialized := range resp.Events {
					ev, err := blockchainevent.NewFromSerialized(serialized)
					if err != nil {
						log.Printf("deserialize event error: %v", err)
						continue
					}
					events = append(events, ev)
				}
				if err := e.ProcessExternalEvents(events, resp.LatestBlockNumber); err != nil {
					log.Printf("process external events error: %v", err)
					continue
				}
			}
			if resp.LatestBlockNumber >= fetchFrom {
				fetchFrom = resp.LatestBlockNumber + 1
				e.AdvanceLatestBlockNumber(resp.LatestBlockNumber)
			}
		}
	}
}

func parseSolanaTransaction(meta *solana.TxMeta) []*solana.DecodedEvent {
	logEvents := solana.ParseLogsForEvents(meta.LogMessages)
	var cpiData []string
	for _, inner := range meta.InnerInstructions {
		for _, ix := range inner.Instructions {
			if ix.Data != "" {
				cpiData = append(cpiData, ix.Data)
			}
		}
	}
	merged := make([]*solana.DecodedEvent, 0, len(logEvents))
	merged = append(merged, logEvents...)
	merged = append(merged, solana.ParseCpiForEvents(cpiData)...)
	if len(merged) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(merged))
	out := make([]*solana.DecodedEvent, 0, len(merged))
	for _, ev := range merged {
		argsJSON, err := json.Marshal(ev.Args)
		if err != nil {
			argsJSON = nil
		}
		key := ev.Name + ":" + string(argsJSON)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ev)
	}
	return out
}

func buildSolanaEvent(d *solana.DecodedEvent, signature string, slot uint64) (*blockchainevent.BlockchainEvent, error) {
	b, err := json.Marshal(map[string]any{
		"eventName":       d.Name,
		"transactionHash": signature,
		"blockNumber":     slot,
		"args":            d.Args,
	})
	if err != nil {
		return nil, err
	}
	return blockchainevent.NewFromSerialized(string(b))
}
