package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/ericls/certmatic/pkg/webhook"
	"go.uber.org/zap"
)

// MemoryDispatcher is an in-memory webhook event dispatcher.
// Events are queued in a buffered channel and delivered by a background goroutine.
type MemoryDispatcher struct {
	endpoints []webhook.Endpoint
	queue     chan webhook.Event
	client    *http.Client
	logger    *zap.Logger
	cancel    context.CancelFunc
}

// NewMemoryDispatcher creates a dispatcher that delivers events to the given endpoints.
// A background goroutine processes the queue until Destruct is called.
func NewMemoryDispatcher(endpoints []webhook.Endpoint, logger *zap.Logger) *MemoryDispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	d := &MemoryDispatcher{
		endpoints: endpoints,
		queue:     make(chan webhook.Event, 1000),
		client:    &http.Client{Timeout: 10 * time.Second},
		logger:    logger,
		cancel:    cancel,
	}
	go d.deliveryLoop(ctx)
	return d
}

// Dispatch enqueues an event for asynchronous delivery.
// If the queue is full the event is dropped and a warning is logged.
func (d *MemoryDispatcher) Dispatch(event webhook.Event) {
	select {
	case d.queue <- event:
		d.logger.Debug("enqueued webhook event",
			zap.String("event_type", string(event.Type)),
			zap.Time("timestamp", event.Timestamp))
	default:
		d.logger.Warn("webhook queue full, dropping event",
			zap.String("event_type", string(event.Type)))
	}
}

// Destruct stops the background delivery goroutine.
func (d *MemoryDispatcher) Destruct() error {
	// Drain the queue before stopping
	go func() {
		for event := range d.queue {
			d.deliver(context.Background(), event)
		}
	}()
	close(d.queue)
	d.cancel()
	return nil
}

func (d *MemoryDispatcher) deliveryLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done(): // Done is cancel, cancel is Done
			return
		case event := <-d.queue:
			d.deliver(ctx, event)
		}
	}
}

func (d *MemoryDispatcher) deliver(ctx context.Context, event webhook.Event) {
	body, err := json.Marshal(event)
	if err != nil {
		d.logger.Error("failed to marshal webhook event", zap.Error(err))
		return
	}

	var wg sync.WaitGroup
	for _, ep := range d.endpoints {
		wg.Add(1)
		go func(ep webhook.Endpoint) {
			defer wg.Done()
			d.deliverToEndpoint(ctx, ep, body)
		}(ep)
	}
	wg.Wait()
}

func (d *MemoryDispatcher) deliverToEndpoint(ctx context.Context, ep webhook.Endpoint, body []byte) {
	const maxAttempts = 3
	backoff := 1 * time.Second

	for attempt := range maxAttempts {
		if ctx.Err() != nil {
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(body))
		if err != nil {
			d.logger.Error("failed to create webhook request",
				zap.String("url", ep.URL), zap.Error(err))
			return
		}
		req.Header.Set("Content-Type", "application/json")

		if ep.SigningKey != "" {
			sig := Sign([]byte(ep.SigningKey), time.Now(), body)
			req.Header.Set(SignatureHeader, sig)
		}

		resp, err := d.client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				d.logger.Debug("webhook delivered successfully",
					zap.String("url", ep.URL),
					zap.Int("attempt", attempt+1),
				)
				return
			}
			d.logger.Warn("webhook delivery got non-2xx",
				zap.String("url", ep.URL),
				zap.Int("status", resp.StatusCode),
				zap.Int("attempt", attempt+1))
		} else {
			d.logger.Warn("webhook delivery failed",
				zap.String("url", ep.URL),
				zap.Error(err),
				zap.Int("attempt", attempt+1))
		}

		if attempt < maxAttempts-1 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
		}
	}
}

var _ webhook.Dispatcher = (*MemoryDispatcher)(nil)
