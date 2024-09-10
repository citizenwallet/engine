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

func TestTopic_convertHashToGoType(t *testing.T) {
	tests := []struct {
		name     string
		hash     common.Hash
		topic    Topic
		expected interface{}
		wantErr  bool
	}{
		{
			name: "bool true",
			hash: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"),
			topic: Topic{
				Type: "bool",
			},
			expected: true,
		},
		{
			name: "bool false",
			hash: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
			topic: Topic{
				Type: "bool",
			},
			expected: false,
		},
		{
			name: "address",
			hash: common.HexToHash("0x0000000000000000000000005566d6d4df27a6fd7856b7564f81266863ba3ee8"),
			topic: Topic{
				Type: "address",
			},
			expected: common.HexToAddress("0x5566D6D4Df27a6fD7856b7564F81266863Ba3ee8"),
		},
		{
			name: "uint256",
			hash: common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000000a"),
			topic: Topic{
				Type: "uint256",
			},
			expected: big.NewInt(10),
		},
		{
			name: "bytes4",
			hash: common.HexToHash("0x1234567800000000000000000000000000000000000000000000000000000000"),
			topic: Topic{
				Type: "bytes4",
			},
			expected: []byte{0x12, 0x34, 0x56, 0x78},
		},
		{
			name: "string",
			hash: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000020"),
			topic: Topic{
				Type: "string",
			},
			expected: "0x0000000000000000000000000000000000000000000000000000000000000020",
		},
		{
			name: "unsupported type",
			hash: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
			topic: Topic{
				Type: "unsupported",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.topic.convertHashToValue(tt.hash)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, tt.topic.Value)
			}
		})
	}
}

func TestParseTopicsFromHashes(t *testing.T) {
	// Create a mock Event for ERC20 Transfer
	event := &Event{
		Name:           "Transfer",
		EventSignature: "Transfer(from indexed address,to indexed address,value uint256)",
	}

	rawABI := `[{"name":"Transfer","type":"event","inputs":[{"name":"from","type":"address", "indexed":true},{"name":"to","type":"address", "indexed":true},{"name":"value","type":"uint256", "indexed":false}]}]`

	abi, err := event.ConstructABIFromEventSignature()
	if err != nil {
		t.Fatal(err)
	}

	assert.JSONEq(t, rawABI, abi)

	// Create mock topic hashes
	topicHashes := []common.Hash{
		common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"), // Transfer event signature
		common.HexToHash("0x000000000000000000000000a1e4380a3b1f749673e270229993ee55f35663b4"), // from address
		common.HexToHash("0x000000000000000000000000bcd4042de499d14e55001ccbb24a551f3b954096"), // to address
	}

	data := common.Hex2Bytes("00000000000000000000000000000000000000000000000000000000000186a0")

	// Parse topics
	topics, err := ParseTopicsFromHashes(event, topicHashes, data)
	if err != nil {
		t.Fatal(err)
	}

	// Assert no error
	assert.NoError(t, err)

	// Assert correct number of topics
	assert.Equal(t, 4, len(topics))

	// Assert event signature topic
	assert.Equal(t, "topic", topics[0].Name)
	assert.Equal(t, "bytes32", topics[0].Type)
	assert.Equal(t, topicHashes[0], topics[0].Value)

	// Assert 'from' address topic
	assert.Equal(t, "from", topics[1].Name)
	assert.Equal(t, "address", topics[1].Type)
	assert.Equal(t, common.HexToAddress("0xa1e4380a3b1f749673e270229993ee55f35663b4"), topics[1].Value)

	// Assert 'to' address topic
	assert.Equal(t, "to", topics[2].Name)
	assert.Equal(t, "address", topics[2].Type)
	assert.Equal(t, common.HexToAddress("0xbcd4042de499d14e55001ccbb24a551f3b954096"), topics[2].Value)
}
