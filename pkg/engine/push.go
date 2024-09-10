package engine

import (
	"encoding/json"
	"fmt"
)

type PushToken struct {
	Token   string
	Account string
}

type PushMessage struct {
	Tokens []*PushToken
	Title  string
	Body   string
	Data   []byte
	Silent bool
}

type PushDescription struct {
	Description string `json:"description"`
}

// sending
const PushMessageSendingAnonymousDescriptionTitle = "Receiving %s %s (%s)..."
const PushMessageSendingAnonymousDescriptionBody = "%s"
const PushMessageSendingAnonymousTitle = "%s"
const PushMessageSendingAnonymousBody = "Receiving %s %s..."

// success
const PushMessageAnonymousDescriptionTitle = "%s %s (%s) received"
const PushMessageAnonymousDescriptionBody = "%s"
const PushMessageAnonymousTitle = "%s"
const PushMessageAnonymousBody = "%s %s received"

const PushMessageTitle = "%s - %s"
const PushMessageBody = "%s %s received from %s"

func parseDescriptionFromData(data *json.RawMessage) *string {
	var desc PushDescription
	err := json.Unmarshal(*data, &desc)
	if err != nil {
		return nil
	}

	return &desc.Description
}

func NewAnonymousPushMessage(token []*PushToken, community, amount, symbol string, tx *Log) *PushMessage {
	mtx, err := json.Marshal(tx)
	if err != nil {
		mtx = nil
	}

	silent := false

	title := ""
	description := ""
	switch tx.Status {
	case LogStatusSending:
		title = fmt.Sprintf(PushMessageSendingAnonymousTitle, community)
		description = fmt.Sprintf(PushMessageSendingAnonymousBody, amount, symbol)
		if descriptionData := parseDescriptionFromData(tx.ExtraData); descriptionData != nil {
			title = fmt.Sprintf(PushMessageSendingAnonymousDescriptionTitle, amount, community, symbol)
			description = fmt.Sprintf(PushMessageSendingAnonymousDescriptionBody, *descriptionData)
		}
	case LogStatusPending:
		silent = true
	case LogStatusSuccess:
		title = fmt.Sprintf(PushMessageAnonymousTitle, community)
		description = fmt.Sprintf(PushMessageAnonymousBody, amount, symbol)
		if descriptionData := parseDescriptionFromData(tx.ExtraData); descriptionData != nil {
			title = fmt.Sprintf(PushMessageAnonymousDescriptionTitle, amount, community, symbol)
			description = fmt.Sprintf(PushMessageAnonymousDescriptionBody, *descriptionData)
		}
	}

	return &PushMessage{
		Tokens: token,
		Title:  title,
		Body:   description,
		Data:   mtx,
		Silent: silent,
	}
}

func NewSilentPushMessage(token []*PushToken, tx *Log) *PushMessage {
	mtx, err := json.Marshal(tx)
	if err != nil {
		mtx = nil
	}

	return &PushMessage{
		Tokens: token,
		Data:   mtx,
		Silent: true,
	}
}

func NewPushMessage(token []*PushToken, community, name, amount, symbol, username string) *PushMessage {
	return &PushMessage{
		Tokens: token,
		Title:  fmt.Sprintf(PushMessageTitle, community, name),
		Body:   fmt.Sprintf(PushMessageBody, amount, symbol, username),
	}
}
