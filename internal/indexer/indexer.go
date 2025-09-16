package indexer

import (
	"context"
	"errors"
	"fmt"

	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/internal/ws"
	"github.com/citizenwallet/engine/pkg/engine"
)

type ErrIndexing error

var (
	ErrIndexingRecoverable ErrIndexing = errors.New("error indexing recoverable") // an error occurred while indexing but it is not fatal
)

type Indexer struct {
	ctx context.Context
	db  *db.DB
	w   engine.WebhookMessager
	evm engine.EVMRequester

	pools *ws.ConnectionPools
}

func NewIndexer(ctx context.Context, db *db.DB, w engine.WebhookMessager, evm engine.EVMRequester, pools *ws.ConnectionPools) *Indexer {
	return &Indexer{ctx: ctx, db: db, w: w, evm: evm, pools: pools}
}

func (i *Indexer) Start() error {
	evs, err := i.db.EventDB.GetEvents()
	if err != nil {
		return err
	}

	quitAck := make(chan error)

	for _, ev := range evs {
		i.w.Notify(i.ctx, fmt.Sprintf("indexing event %s at %s", ev.Name, ev.Contract))

		go func() {
			err := i.ListenToLogs(ev, quitAck)
			if err != nil {
				quitAck <- err
			}
		}()
	}

	return <-quitAck
}
