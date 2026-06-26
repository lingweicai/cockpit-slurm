package account

import (
	"time"
)

// Event is a bridge message delivered to socket subscribers.
type Event struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// RequestResult is the cached outcome of an idempotent frontend request.
type RequestResult struct {
	RequestID  string      `json:"request_id"`
	Status     string      `json:"status"`
	Message    string      `json:"message,omitempty"`
	Data       interface{} `json:"data,omitempty"`
	Error      string      `json:"error,omitempty"`
	Generation uint64      `json:"generation,omitempty"`
	Timestamp  time.Time   `json:"timestamp"`
}
