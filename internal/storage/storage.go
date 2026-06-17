package storage

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

// Bar is one OHLCV candle for a (symbol, timeframe) at a bar-open time (UTC).
type Bar struct {
	Symbol    string
	Timeframe string
	Time      time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// Store is the persistence boundary the rest of the app depends on.
type Store interface {
	Migrate(ctx context.Context) error
	UpsertBars(ctx context.Context, bars []Bar) error
	// GetBars returns bars ascending by time. limit<=0 returns all; otherwise
	// the most recent `limit` bars (still returned ascending).
	GetBars(ctx context.Context, symbol, timeframe string, limit int) ([]Bar, error)
	LastBarTime(ctx context.Context, symbol, timeframe string) (time.Time, bool, error)
	Ping(ctx context.Context) error
	Close() error
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	// Bound the pool so concurrent /screen requests plus the collector cannot
	// exhaust Postgres max_connections (database/sql defaults to unlimited).
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS bars (
	symbol    TEXT             NOT NULL,
	timeframe TEXT             NOT NULL,
	ts        TIMESTAMPTZ      NOT NULL,
	open      DOUBLE PRECISION NOT NULL,
	high      DOUBLE PRECISION NOT NULL,
	low       DOUBLE PRECISION NOT NULL,
	close     DOUBLE PRECISION NOT NULL,
	volume    DOUBLE PRECISION NOT NULL,
	PRIMARY KEY (symbol, timeframe, ts)
);
CREATE INDEX IF NOT EXISTS bars_symbol_tf_ts ON bars (symbol, timeframe, ts DESC);
`

func (s *PostgresStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *PostgresStore) UpsertBars(ctx context.Context, bars []Bar) error {
	if len(bars) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO bars (symbol, timeframe, ts, open, high, low, close, volume)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (symbol, timeframe, ts) DO UPDATE SET
			open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
			close = EXCLUDED.close, volume = EXCLUDED.volume`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, b := range bars {
		if _, err := stmt.ExecContext(ctx, b.Symbol, b.Timeframe, b.Time.UTC(),
			b.Open, b.High, b.Low, b.Close, b.Volume); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) GetBars(ctx context.Context, symbol, timeframe string, limit int) ([]Bar, error) {
	q := `SELECT symbol, timeframe, ts, open, high, low, close, volume
	      FROM bars WHERE symbol = $1 AND timeframe = $2 ORDER BY ts DESC`
	args := []any{symbol, timeframe}
	if limit > 0 {
		q += " LIMIT $3"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var desc []Bar
	for rows.Next() {
		var b Bar
		if err := rows.Scan(&b.Symbol, &b.Timeframe, &b.Time, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, err
		}
		b.Time = b.Time.UTC()
		desc = append(desc, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	return desc, nil
}

func (s *PostgresStore) LastBarTime(ctx context.Context, symbol, timeframe string) (time.Time, bool, error) {
	var ts time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT ts FROM bars WHERE symbol=$1 AND timeframe=$2 ORDER BY ts DESC LIMIT 1`,
		symbol, timeframe).Scan(&ts)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return ts.UTC(), true, nil
}

func (s *PostgresStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *PostgresStore) Close() error                   { return s.db.Close() }
