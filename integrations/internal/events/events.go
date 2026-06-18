// Package events is the cheap, event-driven push that backs WatchConnections: a
// single Postgres LISTEN connection fans connection-change notifications out to
// in-process subscribers, one per active stream. No polling — the listener
// blocks until Postgres delivers a NOTIFY (design-doc 0009/0010).
package events

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// channel is the Postgres NOTIFY channel connection changes are published on.
// Store.NotifyConnectionChanged sends the tenant id as the payload.
const channel = "connection_changed"

// Broker fans connection-change signals out to per-stream subscribers, keyed by
// tenant. Signals are coalesced: a subscriber's channel is buffered to one, and
// a full buffer is left alone (the pending signal already says "re-read") — so a
// burst of changes costs one wake-up, not one per change.
type Broker struct {
	mu   sync.Mutex
	next int
	subs map[string]map[int]chan struct{}
}

func NewBroker() *Broker {
	return &Broker{subs: make(map[string]map[int]chan struct{})}
}

// Subscribe returns a signal channel for a tenant and an unsubscribe func the
// caller must defer.
func (b *Broker) Subscribe(tenant string) (<-chan struct{}, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan struct{}, 1)
	if b.subs[tenant] == nil {
		b.subs[tenant] = make(map[int]chan struct{})
	}
	b.subs[tenant][id] = ch
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if m := b.subs[tenant]; m != nil {
			delete(m, id)
			if len(m) == 0 {
				delete(b.subs, tenant)
			}
		}
	}
}

// Publish wakes every subscriber for a tenant (non-blocking, coalesced).
func (b *Broker) Publish(tenant string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs[tenant] {
		select {
		case ch <- struct{}{}:
		default: // already has a pending signal — coalesce
		}
	}
}

// Listen holds one LISTEN connection and publishes each notification to the
// broker until ctx is cancelled. It reconnects with backoff on error, so a
// dropped connection self-heals (a missed notification only delays a refresh
// until the next change; streams also send a fresh snapshot on resubscribe).
func Listen(ctx context.Context, connString string, b *Broker) {
	const backoff = 2 * time.Second
	for ctx.Err() == nil {
		if err := listenOnce(ctx, connString, b); err != nil && ctx.Err() == nil {
			slog.Error("listen_connection_lost", "error", err.Error())
			select {
			case <-ctx.Done():
			case <-time.After(backoff):
			}
		}
	}
}

func listenOnce(ctx context.Context, connString string, b *Broker) error {
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
		return err
	}
	slog.Info("listening_for_connection_changes")
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		b.Publish(n.Payload) // payload = tenant id
	}
}
