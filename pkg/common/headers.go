package common

import (
	"context"

	"github.com/citizenwallet/engine/pkg/engine"
)

// GetContextAddress returns the indexer.ContextKeyAddress from the context
func GetContextAddress(ctx context.Context) (string, bool) {
	addr, ok := ctx.Value(engine.ContextKeyAddress).(string)
	return addr, ok
}
