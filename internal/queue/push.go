package queue

import "github.com/citizenwallet/engine/pkg/engine"

type PushService struct{}

func NewPushService() *PushService {
	return &PushService{}
}

func (p *PushService) Process(messages []engine.Message) (invalid []engine.Message, errors []error) {
	invalid = []engine.Message{}
	errors = []error{}

	println("push service processing messages", len(messages))

	return
}
