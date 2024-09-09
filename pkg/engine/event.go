package engine

import (
	"strconv"
	"strings"
	"time"
)

type Event struct {
	Contract       string    `json:"contract"`
	EventSignature string    `json:"event_signature"`
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Parse human readable event signature
// Example: Transfer(from address, to address, value uint256)
// Returns: ("Transfer", ["from", "to", "value"], ["address", "address", "uint256"])
//
// Example: Transfer(address,address,uint256)
// Returns: ("Transfer", ["0", "1", "2"], ["address", "address", "uint256"])
func (e *Event) ParseEventSignature() (string, []string, []string) {
	parts := strings.Split(e.EventSignature, "(")
	eventName := parts[0]

	argNames := []string{}
	argTypes := []string{}

	rawArgs := strings.TrimSuffix(parts[1], ")")
	argParts := strings.Split(rawArgs, ",")

	for i, arg := range argParts {
		arg = strings.TrimSpace(arg)
		parts := strings.Fields(arg)

		if len(parts) == 2 {
			// Named argument
			argNames = append(argNames, parts[0])
			argTypes = append(argTypes, parts[1])
		} else if len(parts) == 1 {
			// Unnamed argument
			argNames = append(argNames, strconv.Itoa(i))
			argTypes = append(argTypes, parts[0])
		}
	}

	return eventName, argNames, argTypes
}
