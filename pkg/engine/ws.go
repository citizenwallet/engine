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

type WSMessageCreator interface {
	ToWSMessage(t WSMessageType) *WSMessageLog
}

func (l *Log) ToWSMessage(t WSMessageType) *WSMessageLog {
	poolTopic := l.GetPoolTopic()
	if poolTopic == nil {
		return nil
	}

	b := l.ToJSON()
	if b == nil {
		return nil
	}

	return &WSMessageLog{
		WSMessage: WSMessage{
			PoolID: *poolTopic,
			Type:   t,
			ID:     l.Hash,
		},
		DataType: WSMessageDataTypeLog,
		Data:     *l,
	}
}
