package engine

import (
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

type Topic struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type Topics []Topic

func (t *Topics) String() string {
	ts := make([]string, len(*t))
	for i, topic := range *t {
		ts[i] = fmt.Sprintf("%s: %s", topic.Name, topic.Value)
	}

	return strings.Join(ts, ", ")
}

func (t Topics) MarshalJSON() ([]byte, error) {
	m := map[string]any{}

	for _, topic := range t {
		m[topic.Name] = topic.valueToJsonParseable()
	}

	return json.Marshal(m)
}

func (t *Topic) valueToJsonParseable() any {
	switch v := t.Value.(type) {
	case bool, string:
		return v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v
	case *big.Int:
		return v.String()
	case []byte:
		return base64.StdEncoding.EncodeToString(v)
	case common.Address:
		return v.Hex()
	default:
		return v
	}
}

func (t Topics) Value() (driver.Value, error) {
	jsonData, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func (t *Topics) GenerateTopicQuery(start int) (string, []any) {
	topicQuery := `
		`
	args := []any{}
	for _, topic := range *t {
		topicQuery += fmt.Sprintf("data->>'%s' = $%d AND ", topic.Name, start)
		args = append(args, topic.Value)
		start++
	}
	topicQuery += `
		`
	return topicQuery, args
}
