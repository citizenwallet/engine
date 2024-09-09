package engine

import "time"

type Event struct {
	Contract       string    `json:"contract"`
	EventSignature string    `json:"event_signature"`
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
