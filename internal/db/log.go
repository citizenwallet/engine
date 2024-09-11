package db

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/citizenwallet/engine/pkg/common"
	"github.com/citizenwallet/engine/pkg/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LogDB struct {
	ctx    context.Context
	suffix string
	db     *pgxpool.Pool
	rdb    *pgxpool.Pool
}

// NewLogDB creates a new DB
func NewLogDB(ctx context.Context, db, rdb *pgxpool.Pool, name string) (*LogDB, error) {
	txdb := &LogDB{
		ctx:    ctx,
		suffix: name,
		db:     db,
		rdb:    rdb,
	}

	return txdb, nil
}

// createLogTable creates a table dest store logs in the given db
func (db *LogDB) CreateLogTable() error {
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS t_logs_%s(
		hash TEXT NOT NULL PRIMARY KEY,
		tx_hash text NOT NULL,
		created_at timestamp NOT NULL DEFAULT current_timestamp,
		updated_at timestamp NOT NULL DEFAULT current_timestamp,
		nonce integer NOT NULL,
		sender text NOT NULL,
		dest text NOT NULL,
		value text NOT NULL,
		data jsonb DEFAULT NULL,
		extra_data jsonb DEFAULT NULL,
		status text NOT NULL DEFAULT 'success'
	);
	`, db.suffix))

	return err
}

// createLogTableIndexes creates the indexes for logs in the given db
func (db *LogDB) CreateLogTableIndexes() error {
	suffix := common.ShortenName(db.suffix, 6)

	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	CREATE INDEX IF NOT EXISTS idx_logs_%s_tx_hash ON t_logs_%s (tx_hash);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	// filtering on contract address
	_, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	CREATE INDEX IF NOT EXISTS idx_logs_%s_dest ON t_logs_%s (dest);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	// filtering on event topic for a given contract
	_, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	CREATE INDEX IF NOT EXISTS idx_logs_%s_dest_date ON t_logs_%s (dest, created_at);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	// filtering on event topic for a given contract for a range of dates
	_, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	CREATE INDEX IF NOT EXISTS idx_logs_%s_dest_topic_date ON t_logs_%s (dest, (data->>'topic'), created_at);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	// filtering by address [CANNOT DO THIS ANYMORE]
	// _, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	// CREATE INDEX IF NOT EXISTS idx_logs_%s_to_addr ON t_logs_%s (to_addr);
	// `, suffix, db.suffix))
	// if err != nil {
	// 	return err
	// }

	// _, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	// CREATE INDEX IF NOT EXISTS idx_logs_%s_from_addr ON t_logs_%s (from_addr);
	// `, suffix, db.suffix))
	// if err != nil {
	// 	return err
	// }

	// // single-token queries
	// _, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	// CREATE INDEX IF NOT EXISTS idx_logs_%s_date_from_token_id_from_addr_simple ON t_logs_%s (created_at, token_id, from_addr);
	// `, suffix, db.suffix))
	// if err != nil {
	// 	return err
	// }

	// _, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	// CREATE INDEX IF NOT EXISTS idx_logs_%s_date_from_token_id_to_addr_simple ON t_logs_%s (created_at, token_id, to_addr);
	// `, suffix, db.suffix))
	// if err != nil {
	// 	return err
	// }

	// // sending queries
	// _, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	// CREATE INDEX IF NOT EXISTS idx_logs_%s_status_date_from_tx_hash ON t_logs_%s (status, created_at, tx_hash);
	// `, suffix, db.suffix))
	// if err != nil {
	// 	return err
	// }

	// // finding optimistic transactions
	// _, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	// 	CREATE INDEX IF NOT EXISTS idx_logs_%s_to_addr_from_addr_value ON t_logs_%s (to_addr, from_addr, value);
	// 	`, suffix, db.suffix))
	// if err != nil {
	// 	return err
	// }

	return nil
}

// AddLog adds a log dest the db
func (db *LogDB) AddLog(lg *engine.Log) error {

	// insert log on conflict do nothing
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	INSERT INTO t_logs_%s (hash, tx_hash, nonce, sender, dest, value, data, extra_data, status, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	ON CONFLICT (hash) DO NOTHING
	`, db.suffix), lg.Hash, lg.TxHash, lg.Nonce, lg.To, lg.Value.String(), lg.Data, lg.ExtraData, lg.Status, lg.CreatedAt, lg.UpdatedAt)

	return err
}

// AddLogs adds a list of logs dest the db
func (db *LogDB) AddLogs(lg []*engine.Log) error {

	for _, t := range lg {
		_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
			INSERT INTO t_logs_%s (hash, tx_hash, nonce, sender, dest, value, data, extra_data, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (hash) DO UPDATE SET
				tx_hash = EXCLUDED.tx_hash,
				nonce = EXCLUDED.nonce,
				sender = EXCLUDED.sender,
				dest = EXCLUDED.dest,
				value = EXCLUDED.value,
				data = COALESCE(EXCLUDED.data, t_logs_%s.data),
				extra_data = COALESCE(EXCLUDED.extra_data, t_logs_%s.extra_data),
				status = EXCLUDED.status,
				created_at = EXCLUDED.created_at,
				updated_at = EXCLUDED.updated_at
			`, db.suffix, db.suffix, db.suffix), t.Hash, t.TxHash, t.Nonce, t.Sender, t.To, t.Value.String(), t.Data, t.ExtraData, t.Status, t.CreatedAt, t.UpdatedAt)
		if err != nil {
			return err
		}
	}

	return nil
}

// SetStatus sets the status of a log dest pending
func (db *LogDB) SetStatus(status, hash string) error {
	// if status is success, don't update
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	UPDATE t_logs_%s SET status = $1 WHERE hash = $2 AND status != 'success'
	`, db.suffix), status, hash)

	return err
}

// RemoveLog removes a sending log from the db
func (db *LogDB) RemoveLog(hash string) error {
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	DELETE FROM t_logs_%s WHERE hash = $1 AND status != 'success'
	`, db.suffix), hash)

	return err
}

// RemoveOldInProgressLogs removes any log that is not success or fail from the db
func (db *LogDB) RemoveOldInProgressLogs() error {
	old := time.Now().UTC().Add(-30 * time.Second)

	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	DELETE FROM t_logs_%s WHERE created_at <= $1 AND status IN ('sending', 'pending')
	`, db.suffix), old)

	return err
}

// GetLog returns the log for a given hash
func (db *LogDB) GetLog(hash string) (*engine.Log, error) {
	var log engine.Log
	var value string

	row := db.rdb.QueryRow(db.ctx, fmt.Sprintf(`
		SELECT hash, tx_hash, created_at, updated_at, nonce, sender, dest, value, data, extra_data, status
		FROM t_logs_%s
		WHERE hash = $1
		`, db.suffix), hash)

	err := row.Scan(&log.Hash, &log.TxHash, &log.CreatedAt, &log.UpdatedAt, &log.Nonce, &log.Sender, &log.To, &value, &log.Data, &log.ExtraData, &log.Status)
	if err != nil {
		return nil, err
	}

	log.Value = new(big.Int)
	log.Value.SetString(value, 10)

	return &log, nil
}

// GetAllPaginatedLogs returns the logs paginated
func (db *LogDB) GetAllPaginatedLogs(contract string, signature string, maxDate time.Time, limit, offset int) ([]*engine.Log, error) {
	logs := []*engine.Log{}

	query := fmt.Sprintf(`
	SELECT hash, tx_hash, created_at, updated_at, nonce, sender, dest, value, data, extra_data, status
	FROM t_logs_%s
	WHERE dest = $1 AND data->>'topic' = $2 AND created_at <= $3
	ORDER BY created_at DESC
	LIMIT $4 OFFSET $5
	`, db.suffix)

	args := []any{contract, signature, maxDate, limit, offset}

	rows, err := db.rdb.Query(db.ctx, query, args...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return logs, nil
		}

		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var log engine.Log
		var value string

		err := rows.Scan(&log.Hash, &log.TxHash, &log.CreatedAt, &log.UpdatedAt, &log.Nonce, &log.Sender, &log.To, &value, &log.Data, &log.ExtraData, &log.Status)
		if err != nil {
			return nil, err
		}

		log.Value = new(big.Int)
		log.Value.SetString(value, 10)

		logs = append(logs, &log)
	}

	return logs, nil
}

// GetPaginatedLogs returns the logs for a given from_addr or to_addr paginated
func (db *LogDB) GetPaginatedLogs(contract string, signature string, maxDate time.Time, dataFilters, dataFilters2 map[string]any, limit, offset int) ([]*engine.Log, error) {
	logs := []*engine.Log{}

	query := fmt.Sprintf(`
		SELECT hash, tx_hash, created_at, updated_at, nonce, sender, dest, value, data, extra_data, status
		FROM t_logs_%s
		WHERE dest = $1 AND data->>'topic' = $2 AND created_at <= $3
		`, db.suffix)

	args := []any{contract, signature, maxDate}

	orderLimit := `
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
		`

	if len(dataFilters) > 0 {
		topicQuery, topicArgs := engine.GenerateJSONBQuery(len(args)+1, dataFilters)

		query += `AND `
		query += topicQuery

		args = append(args, topicArgs...)

		if len(dataFilters2) > 0 {
			// I'm being lazy here, could be dynamic
			query += fmt.Sprintf(`
				UNION ALL
				SELECT hash, tx_hash, created_at, updated_at, nonce, sender, dest, value, data, extra_data, status
				FROM t_logs_%s
				WHERE dest = $%d AND data->>'topic' = $%d AND created_at <= $%d
				`, db.suffix, len(args)+1, len(args)+2, len(args)+3)

			args = append(args, contract, signature, maxDate)

			topicQuery2, topicArgs2 := engine.GenerateJSONBQuery(len(args)+1, dataFilters2)

			query += `AND `
			query += topicQuery2

			args = append(args, topicArgs2...)
		}

		argsLength := len(args)

		orderLimit = fmt.Sprintf(`
			ORDER BY created_at DESC LIMIT $%d OFFSET $%d
			`, argsLength+1, argsLength+2)
	}

	args = append(args, limit, offset)

	query += orderLimit

	rows, err := db.rdb.Query(db.ctx, query, args...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return logs, nil
		}

		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var log engine.Log
		var value string

		err := rows.Scan(&log.Hash, &log.TxHash, &log.CreatedAt, &log.UpdatedAt, &log.Nonce, &log.Sender, &log.To, &value, &log.Data, &log.ExtraData, &log.Status)
		if err != nil {
			return nil, err
		}

		log.Value = new(big.Int)
		log.Value.SetString(value, 10)

		logs = append(logs, &log)
	}

	return logs, nil
}

// GetNewLogs returns the logs for a given from_addr or to_addr from a given date
func (db *LogDB) GetAllNewLogs(contract string, signature string, fromDate time.Time, dataFilters map[string]any, limit, offset int) ([]*engine.Log, error) {
	logs := []*engine.Log{}

	query := fmt.Sprintf(`
		SELECT hash, tx_hash, created_at, nonce, sender, dest, value, data, extra_data, status
		FROM t_logs_%s
		WHERE dest = $1 AND data->>'topic' = $2 AND created_at >= $3
		`, db.suffix)

	args := []any{contract, signature, fromDate}

	orderLimit := `
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
		`
	if len(dataFilters) > 0 {
		topicQuery, topicArgs := engine.GenerateJSONBQuery(len(args)+1, dataFilters)

		query += `AND `
		query += topicQuery

		args = append(args, topicArgs...)

		argsLength := len(args)

		orderLimit = fmt.Sprintf(`
			ORDER BY created_at DESC LIMIT $%d OFFSET $%d
			`, argsLength+1, argsLength+2)
	}

	args = append(args, limit, offset)

	query += orderLimit

	rows, err := db.rdb.Query(db.ctx, query, args...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return logs, nil
		}

		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var log engine.Log
		var value string

		err := rows.Scan(&log.Hash, &log.TxHash, &log.CreatedAt, &log.Nonce, &log.Sender, &log.To, &value, &log.Data, &log.ExtraData, &log.Status)
		if err != nil {
			return nil, err
		}

		log.Value = new(big.Int)
		log.Value.SetString(value, 10)

		logs = append(logs, &log)
	}

	return logs, nil
}

// GetNewLogs returns the logs for a given from_addr or to_addr from a given date
func (db *LogDB) GetNewLogs(contract string, signature string, fromDate time.Time, dataFilters, dataFilters2 map[string]any, limit, offset int) ([]*engine.Log, error) {
	logs := []*engine.Log{}

	query := fmt.Sprintf(`
		SELECT hash, tx_hash, created_at, nonce, sender, dest, value, data, extra_data, status
		FROM t_logs_%s
		WHERE dest = $1 AND created_at >= $2
		`, db.suffix)

	args := []any{contract, fromDate}

	orderLimit := `
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
		`
	if len(dataFilters) > 0 {
		topicQuery, topicArgs := engine.GenerateJSONBQuery(len(args)+1, dataFilters)

		query += `AND `
		query += topicQuery

		args = append(args, topicArgs...)

		if len(dataFilters2) > 0 {
			// I'm being lazy here, could be dynamic
			query += fmt.Sprintf(`
				UNION ALL
				SELECT hash, tx_hash, created_at, nonce, sender, dest, value, data, extra_data, status
				FROM t_logs_%s
				WHERE dest = $%d AND created_at >= $%d
				`, db.suffix, len(args)+1, len(args)+2)

			args = append(args, contract, fromDate)

			topicQuery2, topicArgs2 := engine.GenerateJSONBQuery(len(args)+1, dataFilters2)

			query += `AND `
			query += topicQuery2

			args = append(args, topicArgs2...)
		}

		argsLength := len(args)

		orderLimit = fmt.Sprintf(`
			ORDER BY created_at DESC LIMIT $%d OFFSET $%d
			`, argsLength+1, argsLength+2)
	}

	args = append(args, limit, offset)

	query += orderLimit

	rows, err := db.rdb.Query(db.ctx, query, args...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return logs, nil
		}

		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var log engine.Log
		var value string

		err := rows.Scan(&log.Hash, &log.TxHash, &log.CreatedAt, &log.Nonce, &log.Sender, &log.To, &value, &log.Data, &log.ExtraData, &log.Status)
		if err != nil {
			return nil, err
		}

		log.Value = new(big.Int)
		log.Value.SetString(value, 10)

		logs = append(logs, &log)
	}

	return logs, nil
}

// UpdateLogsWithDB returns the logs with data updated from the db
func (db *LogDB) UpdateLogsWithDB(txs []*engine.Log) ([]*engine.Log, error) {
	if len(txs) == 0 {
		return txs, nil
	}

	// Convert the log hashes dest a comma-separated string
	hashStr := ""
	for _, lg := range txs {
		// if last item, don't add a trailing comma
		if lg == txs[len(txs)-1] {
			hashStr += fmt.Sprintf("('%s')", lg.Hash)
			continue
		}

		hashStr += fmt.Sprintf("('%s'),", lg.Hash)
	}

	rows, err := db.rdb.Query(db.ctx, fmt.Sprintf(`
		WITH b(hash) AS (
			VALUES
			%s
		)
		SELECT lg.hash, tx_hash, created_at, nonce, sender, dest, value, data, extra_data, status
		FROM t_logs_%s lg
		JOIN b 
		ON lg.hash = b.hash;
		`, hashStr, db.suffix))
	if err != nil {
		if err == pgx.ErrNoRows {
			return txs, nil
		}

		return nil, err
	}
	defer rows.Close()

	mtxs := map[string]*engine.Log{}
	for _, lg := range txs {
		mtxs[lg.Hash] = lg
	}

	for rows.Next() {
		var log engine.Log
		var value string

		err := rows.Scan(&log.Hash, &log.TxHash, &log.CreatedAt, &log.Nonce, &log.Sender, &log.To, &value, &log.Data, &log.ExtraData, &log.Status)
		if err != nil {
			return nil, err
		}

		log.Value = new(big.Int)
		log.Value.SetString(value, 10)

		// check if exists
		if _, ok := mtxs[log.Hash]; !ok {
			continue
		}

		// update the log
		mtxs[log.Hash].Update(&log)
	}

	return txs, nil
}
