package webhook

import "time"

type EventType string

const (
	EventDomainVerified EventType = "domain_verified"
)

type Event struct {
	Type      EventType      `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}
