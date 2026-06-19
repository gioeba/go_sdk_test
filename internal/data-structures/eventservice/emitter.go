package eventservice

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/data-structures/blockchainevent"
)

const (
	retryMinPageSize = 100
	maxRetries       = 20
)

type EmitterDelegate interface {
	Clear()
	StartUpdateListener(ctx context.Context, emitter *BlockchainEventEmitter)
}

type BlockchainEventEmitter struct {
	chainID                 int
	client                  *ethclient.Client
	contractAddr            common.Address
	contractABI             abi.ABI
	depositOnChainUtxosAddr *common.Address
	initialBlockNumber      uint64
	latestBlockNumber       *uint64
	maxPageSize             *uint64
	isServer                bool
	isReady                 bool
	inProgress              bool

	eventsFetchingMutex *sync.Mutex
	eventProcessors     []blockchainevent.EventProcessorFunc
	OnEventsProcessed   func(count int)
	delegate            EmitterDelegate
}

func New(
	chainID int,
	client *ethclient.Client,
	contractAddr common.Address,
	contractABI abi.ABI,
	initialBlockNumber uint64,
	isServer bool,
	delegate EmitterDelegate,
	eventsFetchingMutex *sync.Mutex,
	maxPageSize *uint64,
	depositOnChainUtxosAddr *common.Address,
) *BlockchainEventEmitter {
	if eventsFetchingMutex == nil {
		eventsFetchingMutex = &sync.Mutex{}
	}
	return &BlockchainEventEmitter{
		chainID:                 chainID,
		client:                  client,
		contractAddr:            contractAddr,
		contractABI:             contractABI,
		depositOnChainUtxosAddr: depositOnChainUtxosAddr,
		initialBlockNumber:      initialBlockNumber,
		isServer:                isServer,
		delegate:                delegate,
		eventsFetchingMutex:     eventsFetchingMutex,
		maxPageSize:             maxPageSize,
	}
}

func (e *BlockchainEventEmitter) ChainID() int { return e.chainID }

func (e *BlockchainEventEmitter) LatestBlockNumber() uint64 {
	if e.latestBlockNumber == nil {
		return e.initialBlockNumber
	}
	return *e.latestBlockNumber
}

func (e *BlockchainEventEmitter) IsReady() bool { return e.isReady }

func (e *BlockchainEventEmitter) AdvanceLatestBlockNumber(blockNumber uint64) {
	if e.latestBlockNumber == nil || blockNumber > *e.latestBlockNumber {
		b := blockNumber
		e.latestBlockNumber = &b
	}
}

func (e *BlockchainEventEmitter) ProcessExternalEvents(events []*blockchainevent.BlockchainEvent, latestBlock uint64) error {
	e.eventsFetchingMutex.Lock()
	defer e.eventsFetchingMutex.Unlock()
	if err := e.processEvents(events, latestBlock); err != nil {
		return err
	}
	e.AdvanceLatestBlockNumber(latestBlock)
	return nil
}

func (e *BlockchainEventEmitter) SyncFromAtMost(blockNumber uint64) {
	if e.latestBlockNumber == nil || blockNumber < *e.latestBlockNumber {
		b := blockNumber
		e.latestBlockNumber = &b
	}
}

func (e *BlockchainEventEmitter) AddEventProcessorFunction(fn blockchainevent.EventProcessorFunc) {
	if e.isReady {
		panic("cannot add processor after Init")
	}
	e.eventProcessors = append(e.eventProcessors, fn)
}

func (e *BlockchainEventEmitter) IntervalClear() {
	e.isReady = false
	e.eventProcessors = nil
	e.delegate.Clear()
}

func (e *BlockchainEventEmitter) Init(ctx context.Context) error {
	if e.isReady {
		return fmt.Errorf("already initialized")
	}
	e.isReady = true
	if err := e.RetrieveEvents(ctx, e.LatestBlockNumber()+1, false); err != nil {
		return err
	}
	go e.delegate.StartUpdateListener(ctx, e)
	return nil
}

func (e *BlockchainEventEmitter) GetEventsInRange(ctx context.Context, from, to uint64) ([]*blockchainevent.BlockchainEvent, error) {
	pages := buildPages(from, to, e.maxPageSize)
	var all []*blockchainevent.BlockchainEvent
	for _, p := range pages {
		evs, err := e.getEventsForSingleContract(ctx, e.contractAddr, p[0], p[1], 0)
		if err != nil {
			return nil, err
		}
		all = append(all, evs...)
	}
	if e.depositOnChainUtxosAddr != nil {
		for _, p := range pages {
			evs, err := e.getEventsForSingleContract(ctx, *e.depositOnChainUtxosAddr, p[0], p[1], 0)
			if err != nil {
				return nil, err
			}
			all = append(all, evs...)
		}
	}
	return all, nil
}

func (e *BlockchainEventEmitter) GetLastBlockNumberForEventRequest(ctx context.Context) (uint64, error) {
	latest, err := e.client.BlockNumber(ctx)
	if err != nil {
		return 0, err
	}
	if !e.isServer {
		return latest, nil
	}
	reorgDepth, err := constants.GetReorgDepth(e.chainID)
	if err != nil {
		return 0, err
	}
	safe := latest - reorgDepth + 1
	if cur := e.LatestBlockNumber(); cur > safe {
		return cur, nil
	}
	return safe, nil
}

func (e *BlockchainEventEmitter) RetrieveEvents(ctx context.Context, fromBlock uint64, force bool) error {
	e.eventsFetchingMutex.Lock()
	defer e.eventsFetchingMutex.Unlock()
	if e.inProgress && !force {
		return nil
	}
	e.inProgress = true
	defer func() { e.inProgress = false }()

	lastBlock, err := e.GetLastBlockNumberForEventRequest(ctx)
	if err != nil {
		return fmt.Errorf("get last block: %w", err)
	}
	if lastBlock < fromBlock {
		return nil
	}
	events, err := e.GetEventsInRange(ctx, fromBlock, lastBlock)
	if err != nil {
		return fmt.Errorf("get events in range: %w", err)
	}
	if err := e.processEvents(events, lastBlock); err != nil {
		return err
	}
	e.latestBlockNumber = &lastBlock
	return nil
}

func (e *BlockchainEventEmitter) getEventsForSingleContract(
	ctx context.Context, addr common.Address, from, to uint64, retry int,
) ([]*blockchainevent.BlockchainEvent, error) {
	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{addr},
	}
	logs, err := e.client.FilterLogs(ctx, query)
	if err != nil {
		if retry < maxRetries && to-from > retryMinPageSize {
			mid := (from + to) / 2
			a, err := e.getEventsForSingleContract(ctx, addr, from, mid, retry+1)
			if err != nil {
				return nil, err
			}
			b, err := e.getEventsForSingleContract(ctx, addr, mid+1, to, retry+1)
			if err != nil {
				return nil, err
			}
			return append(a, b...), nil
		}
		return nil, err
	}
	events := make([]*blockchainevent.BlockchainEvent, 0, len(logs))
	for _, log := range logs {
		ev, err := blockchainevent.NewFromLog(log, e.contractABI)
		if err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}

func (e *BlockchainEventEmitter) processEvents(events []*blockchainevent.BlockchainEvent, scannedToBlock uint64) error {
	total := 0
	for _, proc := range e.eventProcessors {
		n, err := proc(events, &scannedToBlock)
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

func buildPages(from, to uint64, maxPage *uint64) [][2]uint64 {
	if maxPage == nil || *maxPage == 0 {
		return [][2]uint64{{from, to}}
	}
	var pages [][2]uint64
	for cur := from; cur <= to; cur += *maxPage {
		end := min(cur+*maxPage-1, to)
		pages = append(pages, [2]uint64{cur, end})
	}
	return pages
}
