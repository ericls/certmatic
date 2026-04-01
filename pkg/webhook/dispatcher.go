package webhook

// Endpoint pairs a webhook URL with an optional signing key.
type Endpoint struct {
	URL        string `json:"url"`
	SigningKey string `json:"signing_key,omitempty"`
}

// DispatcherConfig holds the configuration for the webhook dispatcher.
type DispatcherConfig struct {
	Type      string     `json:"type,omitempty"`
	Endpoints []Endpoint `json:"endpoints,omitempty"`
}

// Dispatcher delivers webhook events to configured URLs.
// Dispatch is fire-and-forget: implementations handle queuing, delivery, and retries.
type Dispatcher interface {
	Dispatch(event Event)
}

// NoopDispatcher silently discards all events. Used when no webhook_dispatcher is configured.
type NoopDispatcher struct{}

func (NoopDispatcher) Dispatch(Event) {}
