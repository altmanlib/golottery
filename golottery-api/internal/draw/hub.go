package draw

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	statsInterval = 5 * time.Second
	channelPrefix = "gl:event:"
)

// Stats is the periodic host heartbeat.
type Stats struct {
	CheckedIn   int64
	DrawVersion int64
}

// Message is one SSE payload shared with Redis.
type Message struct {
	Type        string       `json:"type"`
	DrawVersion int64        `json:"draw_version"`
	CheckedIn   int64        `json:"checked_in,omitempty"`
	Results     []ResultView `json:"results,omitempty"`
}

type subscriber struct {
	ch     chan Message
	cancel context.CancelFunc
}

// Hub fans draw events out to local SSE connections and across instances via Redis.
type Hub struct {
	redis  *redis.Client
	logger *slog.Logger
	stats  func(context.Context, uuid.UUID) (Stats, error)

	mu     sync.Mutex
	local  map[uuid.UUID]map[*subscriber]struct{}
	pubsub map[uuid.UUID]*redis.PubSub
	timers map[uuid.UUID]context.CancelFunc
	closed bool
}

// NewHub builds a Hub. redis may be nil; publish then becomes a no-op beyond local fan-out.
func NewHub(rdb *redis.Client, logger *slog.Logger, stats func(context.Context, uuid.UUID) (Stats, error)) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		redis:  rdb,
		logger: logger,
		stats:  stats,
		local:  map[uuid.UUID]map[*subscriber]struct{}{},
		pubsub: map[uuid.UUID]*redis.PubSub{},
		timers: map[uuid.UUID]context.CancelFunc{},
	}
}

// Subscribe registers a local SSE consumer for eventID. The cancel function removes it.
func (h *Hub) Subscribe(eventID uuid.UUID) (<-chan Message, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		ch := make(chan Message)
		close(ch)
		return ch, func() {}
	}
	_, cancel := context.WithCancel(context.Background())
	sub := &subscriber{ch: make(chan Message, 16), cancel: cancel}
	if h.local[eventID] == nil {
		h.local[eventID] = map[*subscriber]struct{}{}
		h.startPubSubLocked(eventID)
		h.startStatsLocked(eventID)
	}
	h.local[eventID][sub] = struct{}{}
	return sub.ch, func() {
		h.unsubscribe(eventID, sub)
	}
}

// Publish sends msg to local subscribers and to Redis. Redis errors are logged only.
func (h *Hub) Publish(eventID uuid.UUID, msg Message) {
	h.fanout(eventID, msg)
	if h.redis == nil {
		return
	}
	body, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("draw hub encode", "error", err)
		return
	}
	if err := h.redis.Publish(context.Background(), channelPrefix+eventID.String(), body).Err(); err != nil {
		h.logger.Error("draw hub publish", "event", eventID, "error", err)
	}
}

// Close ends every local stream so http.Server.Shutdown can finish.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for eventID, subs := range h.local {
		for sub := range subs {
			sub.cancel()
			close(sub.ch)
		}
		delete(h.local, eventID)
		h.stopPubSubLocked(eventID)
		h.stopStatsLocked(eventID)
	}
}

func (h *Hub) unsubscribe(eventID uuid.UUID, sub *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs := h.local[eventID]
	if _, ok := subs[sub]; !ok {
		return
	}
	delete(subs, sub)
	sub.cancel()
	close(sub.ch)
	if len(subs) == 0 {
		delete(h.local, eventID)
		h.stopPubSubLocked(eventID)
		h.stopStatsLocked(eventID)
	}
}

func (h *Hub) fanout(eventID uuid.UUID, msg Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.local[eventID] {
		select {
		case sub.ch <- msg:
		default:
			// A slow client drops intermediate events; draw_version on the next
			// stats tick makes it reload the snapshot.
		}
	}
}

func (h *Hub) startPubSubLocked(eventID uuid.UUID) {
	if h.redis == nil {
		return
	}
	if _, ok := h.pubsub[eventID]; ok {
		return
	}
	ps := h.redis.Subscribe(context.Background(), channelPrefix+eventID.String())
	h.pubsub[eventID] = ps
	go func() {
		ch := ps.Channel()
		for msg := range ch {
			var payload Message
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				h.logger.Error("draw hub decode", "error", err)
				continue
			}
			h.fanout(eventID, payload)
		}
	}()
}

func (h *Hub) stopPubSubLocked(eventID uuid.UUID) {
	if ps, ok := h.pubsub[eventID]; ok {
		_ = ps.Close()
		delete(h.pubsub, eventID)
	}
}

func (h *Hub) startStatsLocked(eventID uuid.UUID) {
	if _, ok := h.timers[eventID]; ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.timers[eventID] = cancel
	go func() {
		ticker := time.NewTicker(statsInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if h.stats == nil {
					continue
				}
				stats, err := h.stats(ctx, eventID)
				if err != nil {
					h.logger.Error("draw hub stats", "event", eventID, "error", err)
					continue
				}
				h.fanout(eventID, Message{
					Type: "stats", DrawVersion: stats.DrawVersion, CheckedIn: stats.CheckedIn,
				})
			}
		}
	}()
}

func (h *Hub) stopStatsLocked(eventID uuid.UUID) {
	if cancel, ok := h.timers[eventID]; ok {
		cancel()
		delete(h.timers, eventID)
	}
}

// EncodeSSE writes one SSE event body.
func EncodeSSE(eventType string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, body)), nil
}
