package eventservice

import (
	"context"
	"log"
	"time"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/internal/data-structures/blockchainevent"
	"github.com/gioeba/go_sdk_test/types"
)

const clientEventPollInterval = 5 * time.Second

type ClientBlockchainEventEmitter struct {
	eventCategory types.EventCategory
	cancel        context.CancelFunc
}

func NewClientBlockchainEventEmitter(eventCategory types.EventCategory) *ClientBlockchainEventEmitter {
	return &ClientBlockchainEventEmitter{eventCategory: eventCategory}
}

func (d *ClientBlockchainEventEmitter) Clear() {
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
}

func (d *ClientBlockchainEventEmitter) StartUpdateListener(ctx context.Context, emitter *BlockchainEventEmitter) {
	ctx, d.cancel = context.WithCancel(ctx)
	fetchFrom := emitter.LatestBlockNumber() + 1
	go func() {
		ticker := time.NewTicker(clientEventPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				resp, err := api.GetSnapshotServerEvents(ctx, emitter.ChainID(), d.eventCategory, fetchFrom)
				if err != nil {
					log.Printf("snapshot events poll error: %v", err)
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
					if err := emitter.ProcessExternalEvents(events, resp.LatestBlockNumber); err != nil {
						log.Printf("process external events error: %v", err)
						continue
					}
				}
				if resp.LatestBlockNumber >= fetchFrom {
					fetchFrom = resp.LatestBlockNumber + 1
					emitter.AdvanceLatestBlockNumber(resp.LatestBlockNumber)
				}
			}
		}
	}()
}
