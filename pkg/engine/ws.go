package engine

type WSMessageType string

const (
	WSMessageTypeNew    WSMessageType = "new"
	WSMessageTypeUpdate WSMessageType = "update"
	WSMessageTypeRemove WSMessageType = "remove"
)

type WSMessageDataType string

const (
	WSMessageDataTypeLog WSMessageDataType = "log"
)

type WSMessage struct {
	PoolID string        `json:"pool_id"`
	Type   WSMessageType `json:"type"`
	ID     string        `json:"id"`
}

type WSMessageLog struct {
	WSMessage
	DataType WSMessageDataType `json:"data_type"`
	Data     Log               `json:"data"`
}
