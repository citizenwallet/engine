package indexer

import (
	"context"
	"errors"

	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/pkg/engine"
)

type ErrIndexing error

var (
	ErrIndexingRecoverable ErrIndexing = errors.New("error indexing recoverable") // an error occurred while indexing but it is not fatal
)

type Indexer struct {
	ctx context.Context
	db  *db.DB
	evm engine.EVMRequester
}

func NewIndexer(ctx context.Context, db *db.DB, evm engine.EVMRequester) *Indexer {
	return &Indexer{ctx: ctx, db: db, evm: evm}
}

func (i *Indexer) Start() error {
	evs, err := i.db.EventDB.GetEvents()
	if err != nil {
		return err
	}

	quitAck := make(chan error)

	for _, ev := range evs {
		go func() {
			err := i.ListenToLogs(ev, quitAck)
			if err != nil {
				quitAck <- err
			}
		}()
	}

	return <-quitAck
}
