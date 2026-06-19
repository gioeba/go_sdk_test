package eventservice

import (
	"context"
	"log"
	"time"
)

const (
	serverPollInterval = 60 * time.Second
	clientPollInterval = 3500 * time.Millisecond
)

type PollingBlockchainEventEmitter struct {
	isServer bool
	cancel   context.CancelFunc
}

func NewPollingBlockchainEventEmitter(isServer bool) *PollingBlockchainEventEmitter {
	return &PollingBlockchainEventEmitter{isServer: isServer}
}

func (d *PollingBlockchainEventEmitter) Clear() {
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
}

func (d *PollingBlockchainEventEmitter) StartUpdateListener(ctx context.Context, emitter *BlockchainEventEmitter) {
	interval := clientPollInterval
	if d.isServer {
		interval = serverPollInterval
	}
	ctx, d.cancel = context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := emitter.RetrieveEvents(ctx, emitter.LatestBlockNumber(), false); err != nil {
					log.Printf("retrieve events poll error: %v", err)
				}
			}
		}
	}()
}
