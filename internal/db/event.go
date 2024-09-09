package db

import (
	"context"
	"fmt"
	"time"

	"github.com/citizenwallet/engine/pkg/engine"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventDB struct {
	ctx    context.Context
	suffix string
	db     *pgxpool.Pool
	rdb    *pgxpool.Pool
}

// NewEventDB creates a new DB
func NewEventDB(ctx context.Context, db, rdb *pgxpool.Pool, name string) (*EventDB, error) {
	evdb := &EventDB{
		ctx:    ctx,
		suffix: name,
		db:     db,
		rdb:    rdb,
	}

	return evdb, nil
}

// createEventsTable creates a table to store events in the given db
func (db *EventDB) CreateEventsTable(suffix string) error {
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS t_events_%s(
		contract text NOT NULL,
		state text NOT NULL,
		created_at timestamp NOT NULL DEFAULT current_timestamp,
		updated_at timestamp NOT NULL DEFAULT current_timestamp,
		start_block integer NOT NULL,
		last_block integer NOT NULL,
		standard text NOT NULL,
		name text NOT NULL,
		symbol text NOT NULL,
		decimals integer NOT NULL DEFAULT 6,
		UNIQUE (contract, standard)
	);
	`, suffix))

	return err
}

// createEventsTableIndexes creates the indexes for events in the given db
func (db *EventDB) CreateEventsTableIndexes(suffix string) error {
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
    CREATE INDEX IF NOT EXISTS idx_events_%s_state ON t_events_%s (state);
    `, suffix, suffix))
	if err != nil {
		return err
	}

	_, err = db.db.Exec(db.ctx, fmt.Sprintf(`
    CREATE INDEX IF NOT EXISTS idx_events_%s_address_signature ON t_events_%s (contract, standard);
    `, suffix, suffix))
	if err != nil {
		return err
	}

	_, err = db.db.Exec(db.ctx, fmt.Sprintf(`
    CREATE INDEX IF NOT EXISTS idx_events_%s_address_signature_state ON t_events_%s (contract, standard, state);
    `, suffix, suffix))
	if err != nil {
		return err
	}

	return nil
}

// GetEvent gets an event from the db by contract and standard
func (db *EventDB) GetEvent(contract string, standard engine.Standard) (*engine.Event, error) {
	var event engine.Event
	err := db.rdb.QueryRow(db.ctx, fmt.Sprintf(`
	SELECT contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol, decimals
	FROM t_events_%s
	WHERE contract = $1 AND standard = $2
	`, db.suffix), contract, standard).Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Standard, &event.Name, &event.Symbol, &event.Decimals)
	if err != nil {
		return nil, err
	}

	return &event, nil
}

// GetEvents gets all events from the db
func (db *EventDB) GetEvents() ([]*engine.Event, error) {
	rows, err := db.rdb.Query(db.ctx, fmt.Sprintf(`
    SELECT contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol, decimals
    FROM t_events_%s
    ORDER BY created_at ASC
    `, db.suffix))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*engine.Event{}
	for rows.Next() {
		var event engine.Event
		err = rows.Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Standard, &event.Name, &event.Symbol, &event.Decimals)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

// GetOutdatedEvents gets all queued events from the db sorted by created_at
func (db *EventDB) GetOutdatedEvents(currentBlk int64) ([]*engine.Event, error) {
	rows, err := db.rdb.Query(db.ctx, fmt.Sprintf(`
    SELECT contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol, decimals
    FROM t_events_%s
    WHERE last_block < $1
    ORDER BY created_at ASC
    `, db.suffix), currentBlk)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*engine.Event{}
	for rows.Next() {
		var event engine.Event
		err = rows.Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Standard, &event.Name, &event.Symbol, &event.Decimals)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

// GetQueuedEvents gets all queued events from the db sorted by created_at
func (db *EventDB) GetQueuedEvents() ([]*engine.Event, error) {
	rows, err := db.rdb.Query(db.ctx, fmt.Sprintf(`
    SELECT contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol, decimals
    FROM t_events_%s
    WHERE state = $1
    ORDER BY created_at ASC
    `, db.suffix), engine.EventStateQueued)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*engine.Event{}
	for rows.Next() {
		var event engine.Event
		err = rows.Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Standard, &event.Name, &event.Symbol, &event.Decimals)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

// SetEventState sets the state of an event
func (db *EventDB) SetEventState(contract string, standard engine.Standard, state engine.EventState) error {
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
    UPDATE t_events_%s
    SET state = $1, updated_at = $2
    WHERE contract = $3 AND standard = $4
    `, db.suffix), state, time.Now().UTC(), contract, standard)

	return err
}

// SetEventLastBlock sets the last block of an event
func (db *EventDB) SetEventLastBlock(contract string, standard engine.Standard, lastBlock int64) error {
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
    UPDATE t_events_%s
    SET last_block = $1, updated_at = $2
    WHERE contract = $3 AND standard = $4
    `, db.suffix), lastBlock, time.Now().UTC(), contract, standard)

	return err
}

// AddEvent adds an event to the db
func (db *EventDB) AddEvent(contract string, state engine.EventState, startBlk, lastBlk int64, std engine.Standard, name, symbol string, decimals int64) error {
	t := time.Now().UTC()

	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
    INSERT INTO t_events_%s (contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol, decimals)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    ON CONFLICT (contract, standard)
    DO UPDATE SET
        state = EXCLUDED.state,
        updated_at = EXCLUDED.updated_at,
        start_block = EXCLUDED.start_block,
        last_block = EXCLUDED.last_block,
        name = EXCLUDED.name,
        symbol = EXCLUDED.symbol,
        decimals = EXCLUDED.decimals
    `, db.suffix), contract, state, t, t, startBlk, lastBlk, std, name, symbol, decimals)
	if err != nil {
		return err
	}

	return err
}
