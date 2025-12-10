package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"regexp"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/pgxpool"
)

const (
	dbBaseFolder         = "data"
	dbWriterConfigString = "cache=private&_journal=WAL&mode=rwc"
	dbReaderConfigString = "cache=private&_journal=WAL&mode=ro"
)

type DB struct {
	ctx context.Context

	chainID *big.Int
	mu      sync.Mutex
	db      *pgxpool.Pool
	rdb     *pgxpool.Pool

	EventDB     *EventDB
	SponsorDB   *SponsorDB
	LogDB       *LogDB
	UserOpDB    *UserOpDB
	PushTokenDB map[string]*PushTokenDB
}

// NewDB instantiates a new DB
func NewDB(chainID *big.Int, secret, username, password, dbname, port, host, rhost string) (*DB, error) {
	ctx := context.Background()

	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=disable", username, password, dbname, host, port)
	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	err = db.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	evname := chainID.String()

	eventDB, err := NewEventDB(ctx, db, db, evname)
	if err != nil {
		return nil, err
	}

	sponsorDB, err := NewSponsorDB(ctx, db, db, evname, secret)
	if err != nil {
		return nil, err
	}

	datadb, err := NewDataDB(ctx, db, db, evname)
	if err != nil {
		return nil, err
	}

	logDB, err := NewLogDB(ctx, db, db, evname, datadb)
	if err != nil {
		return nil, err
	}

	userOpDB, err := NewUserOpDB(ctx, db, db, evname)
	if err != nil {
		return nil, err
	}

	d := &DB{
		ctx:       ctx,
		chainID:   chainID,
		db:        db,
		rdb:       db,
		EventDB:   eventDB,
		SponsorDB: sponsorDB,
		LogDB:     logDB,
		UserOpDB:  userOpDB,
	}

	// check if db exists before opening, since we use rwc mode
	exists, err := d.EventTableExists(evname)
	if err != nil {
		return nil, err
	}

	if !exists {
		// create table
		err = eventDB.CreateEventsTable(evname)
		if err != nil {
			return nil, err
		}

		// create indexes
		err = eventDB.CreateEventsTableIndexes(evname)
		if err != nil {
			return nil, err
		}
	}

	// check if db exists before opening, since we use rwc mode
	exists, err = d.SponsorTableExists(evname)
	if err != nil {
		return nil, err
	}

	if !exists {
		// create table
		err = sponsorDB.CreateSponsorsTable(evname)
		if err != nil {
			return nil, err
		}

		// create indexes
		err = sponsorDB.CreateSponsorsTableIndexes(evname)
		if err != nil {
			return nil, err
		}
	}

	log.Default().Println("creating transfer db for: ", evname)

	// check if db exists before opening, since we use rwc mode
	exists, err = d.LogTableExists(evname)
	if err != nil {
		return nil, err
	}

	if !exists {
		// create table
		err = d.LogDB.CreateLogTable()
		if err != nil {
			return nil, err
		}

		// create indexes
		err = d.LogDB.CreateLogTableIndexes()
		if err != nil {
			return nil, err
		}
	}

	log.Default().Println("creating data db for: ", evname)

	// check if db exists before opening, since we use rwc mode
	exists, err = d.DataTableExists(evname)
	if err != nil {
		return nil, err
	}

	if !exists {
		// create table
		err = datadb.CreateDataTable()
		if err != nil {
			return nil, err
		}

		// create indexes
		err = datadb.CreateDataTableIndexes()
		if err != nil {
			return nil, err
		}
	}

	log.Default().Println("creating userop db for: ", evname)

	// check if userop table exists before creating
	exists, err = d.UserOpTableExists(evname)
	if err != nil {
		return nil, err
	}

	if !exists {
		// create table
		err = userOpDB.CreateUserOpTable()
		if err != nil {
			return nil, err
		}

		// create indexes
		err = userOpDB.CreateUserOpTableIndexes()
		if err != nil {
			return nil, err
		}
	}

	ptdb := map[string]*PushTokenDB{}

	evs, err := eventDB.GetEvents()
	if err != nil {
		return nil, err
	}

	for _, ev := range evs {
		name, err := d.TableNameSuffix(ev.Contract)
		if err != nil {
			return nil, err
		}

		log.Default().Println("creating push token db for: ", name)

		ptdb[name], err = NewPushTokenDB(ctx, db, db, name)
		if err != nil {
			return nil, err
		}

		// check if db exists before opening, since we use rwc mode
		exists, err = d.PushTokenTableExists(name)
		if err != nil {
			return nil, err
		}

		if !exists {
			// create table
			err = ptdb[name].CreatePushTable()
			if err != nil {
				return nil, err
			}

			// create indexes
			err = ptdb[name].CreatePushTableIndexes()
			if err != nil {
				return nil, err
			}
		}
	}

	d.PushTokenDB = ptdb

	return d, nil
}

// EventTableExists checks if a table exists in the database
func (db *DB) EventTableExists(suffix string) (bool, error) {
	tableName := fmt.Sprintf("t_events_%s", suffix)
	var exists bool
	err := db.rdb.QueryRow(db.ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", tableName).Scan(&exists)
	if err != nil {
		// A database error occurred
		return false, err
	}
	return exists, nil
}

// SponsorTableExists checks if a table exists in the database
func (db *DB) SponsorTableExists(suffix string) (bool, error) {
	tableName := fmt.Sprintf("t_sponsors_%s", suffix)
	var exists bool
	err := db.rdb.QueryRow(db.ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", tableName).Scan(&exists)
	if err != nil {
		// A database error occurred
		return false, err
	}
	return exists, nil
}

// LogTableExists checks if a table exists in the database
func (db *DB) LogTableExists(suffix string) (bool, error) {
	tableName := fmt.Sprintf("t_transfers_%s", suffix)
	var exists bool
	err := db.rdb.QueryRow(db.ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", tableName).Scan(&exists)
	if err != nil {
		// A database error occurred
		return false, err
	}
	return exists, nil
}

// PushTokenTableExists checks if a table exists in the database
func (db *DB) PushTokenTableExists(suffix string) (bool, error) {
	tableName := fmt.Sprintf("t_push_token_%s", suffix)
	var exists bool
	err := db.rdb.QueryRow(db.ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", tableName).Scan(&exists)
	if err != nil {
		// A database error occurred
		return false, err
	}
	return exists, nil
}

// DataTableExists checks if a table exists in the database
func (db *DB) DataTableExists(suffix string) (bool, error) {
	tableName := fmt.Sprintf("t_logs_data_%s", suffix)
	var exists bool
	err := db.rdb.QueryRow(db.ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", tableName).Scan(&exists)
	if err != nil {
		// A database error occurred
		return false, err
	}
	return exists, nil
}

// UserOpTableExists checks if the user operations table exists in the database
func (db *DB) UserOpTableExists(suffix string) (bool, error) {
	tableName := fmt.Sprintf("t_userops_%s", suffix)
	var exists bool
	err := db.rdb.QueryRow(db.ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", tableName).Scan(&exists)
	if err != nil {
		// A database error occurred
		return false, err
	}
	return exists, nil
}

// TableNameSuffix returns the name of the transfer db for the given contract
func (d *DB) TableNameSuffix(contract string) (string, error) {
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")

	suffix := fmt.Sprintf("%v_%s", d.chainID, strings.ToLower(contract))

	if !re.MatchString(contract) {
		return suffix, errors.New("bad contract address")
	}

	return suffix, nil
}

// GetPushTokenDB returns true if the push token db for the given contract exists, returns the db if it exists
func (d *DB) GetPushTokenDB(contract string) (*PushTokenDB, bool) {
	name, err := d.TableNameSuffix(contract)
	if err != nil {
		return nil, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	ptdb, ok := d.PushTokenDB[name]
	if !ok {
		return nil, false
	}
	return ptdb, true
}

// AddPushTokenDB adds a new push token db for the given contract
func (d *DB) AddPushTokenDB(contract string) (*PushTokenDB, error) {
	name, err := d.TableNameSuffix(contract)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if ptdb, ok := d.PushTokenDB[name]; ok {
		return ptdb, nil
	}
	ptdb, err := NewPushTokenDB(d.ctx, d.db, d.rdb, name)
	if err != nil {
		return nil, err
	}
	d.PushTokenDB[name] = ptdb
	return ptdb, nil
}

// Close closes the db and all its transfer and push dbs
func (d *DB) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i := range d.PushTokenDB {
		delete(d.PushTokenDB, i)
	}

	d.db.Close()
	d.rdb.Close()
}
