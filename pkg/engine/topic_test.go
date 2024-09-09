package engine

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestTopicsMarshalJSON(t *testing.T) {
	tests := []Topics{
		{ // ERC20 Transfer
			{Name: "from", Type: "address", Value: common.HexToAddress("0x1234567890123456789012345678901234567890")},
			{Name: "to", Type: "address", Value: common.HexToAddress("0x1234567890123456789012345678901234567890")},
			{Name: "value", Type: "uint256", Value: big.NewInt(1000000)},
		},
		{ // ERC721 Transfer
			{Name: "from", Type: "address", Value: common.HexToAddress("0x1234567890123456789012345678901234567890")},
			{Name: "to", Type: "address", Value: common.HexToAddress("0x1234567890123456789012345678901234567890")},
			{Name: "tokenId", Type: "uint256", Value: big.NewInt(1)},
		},
	}

	expectedJSON := []string{
		`{
			"from": "0x1234567890123456789012345678901234567890",
			"to": "0x1234567890123456789012345678901234567890",
			"value": "1000000"
		}`,
		`{
			"from": "0x1234567890123456789012345678901234567890",
			"to": "0x1234567890123456789012345678901234567890",
			"tokenId": "1"
		}`,
	}

	for i, tt := range tests {
		jsonData, err := json.Marshal(tt)
		assert.NoError(t, err)

		assert.JSONEq(t, expectedJSON[i], string(jsonData))
	}

}
