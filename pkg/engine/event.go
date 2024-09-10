package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type Event struct {
	Contract       string    `json:"contract"`
	EventSignature string    `json:"event_signature"`
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ArgType struct {
	Name    string
	Indexed bool
}

// Parse human readable event signature
// Example: Transfer(from address, to address, value uint256)
// Returns: ("Transfer", ["from", "to", "value"], [{Name: "address", Indexed: false}, {Name: "address", Indexed: false}, {Name: "uint256", Indexed: false}])
//
// Example: Transfer(address,address,uint256)
// Returns: ("Transfer", ["0", "1", "2"], [{Name: "address", Indexed: false}, {Name: "address", Indexed: false}, {Name: "uint256", Indexed: false}])
//
// Example: Transfer(from indexed address, to indexed address, value uint256)
// Returns: ("Transfer", ["from", "to", "value"], [{Name: "address", Indexed: true}, {Name: "address", Indexed: true}, {Name: "uint256", Indexed: false}])
//
// Example: Transfer(indexed address, indexed address, uint256)
// Returns: ("Transfer", ["0", "1", "2"], [{Name: "address", Indexed: true}, {Name: "address", Indexed: true}, {Name: "uint256", Indexed: false}])
func (e *Event) ParseEventSignature() (string, []string, []ArgType) {
	if e.EventSignature == "" {
		return "", []string{}, []ArgType{}
	}

	parts := strings.Split(e.EventSignature, "(")
	eventName := parts[0]

	argNames := []string{}
	argTypes := []ArgType{}

	rawArgs := strings.TrimSuffix(parts[1], ")")
	argParts := strings.Split(rawArgs, ",")

	for i, arg := range argParts {
		arg = strings.TrimSpace(arg)
		parts := strings.Fields(arg)

		isIndexed := false
		var argName, argType string

		if len(parts) >= 2 && parts[0] == "indexed" {
			// Indexed argument
			isIndexed = true
			parts = parts[1:] // Remove "indexed" from parts
		}

		if len(parts) >= 2 && parts[1] == "indexed" {
			// Indexed argument
			isIndexed = true
			parts = append(parts[:1], parts[2:]...) // Remove "indexed" from the middle
		}

		if len(parts) == 2 {
			// Named argument
			argName = parts[0]
			argType = parts[1]
		} else if len(parts) == 1 {
			// Unnamed argument
			argName = strconv.Itoa(i)
			argType = parts[0]
		}

		argNames = append(argNames, argName)
		argTypes = append(argTypes, ArgType{Name: argType, Indexed: isIndexed})
	}

	return eventName, argNames, argTypes
}

func (e *Event) GetTopic0FromEventSignature() common.Hash {
	name, _, argTypes := e.ParseEventSignature()
	if name == "" || len(argTypes) == 0 {
		return common.Hash{}
	}

	types := make([]string, len(argTypes))
	for i, argType := range argTypes {
		types[i] = argType.Name
	}

	funcSig := fmt.Sprintf("%s(%s)", name, strings.Join(types, ","))

	return crypto.Keccak256Hash([]byte(funcSig))
}

// ConstructABIFromEventSignature constructs an ABI from an event signature
// Example: Transfer(from address, to address, value uint256)
// Returns: {"name":"Transfer","type":"event","inputs":[{"name":"from","type":"address","indexed":false},{"name":"to","type":"address","indexed":false},{"name":"value","type":"uint256","indexed":false}]}
//
// Example: Transfer(from indexed address, to indexed address, value uint256)
// Returns: {"name":"Transfer","type":"event","inputs":[{"name":"from","type":"address", "indexed": true},{"name":"to","type":"address", "indexed": true},{"name":"value","type":"uint256", "indexed": false}]}
func (e *Event) ConstructABIFromEventSignature() (string, error) {
	name, args, argTypes := e.ParseEventSignature()
	if name == "" || len(args) == 0 || len(argTypes) == 0 {
		return "", fmt.Errorf("event name is required")
	}

	abi := fmt.Sprintf(`[{"name":"%s","type":"event","inputs":[`, name)
	for i, arg := range args {
		abi += fmt.Sprintf(`{"name":"%s","type":"%s","indexed":%t}`, arg, argTypes[i].Name, argTypes[i].Indexed)

		// add comma if not last argument
		if i < len(args)-1 {
			abi += ","
		}
	}
	abi += `]}]`

	return abi, nil
}
