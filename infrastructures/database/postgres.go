package database

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"time"

	"github.com/okieraised/go-common/cerrors"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Config defines connection + pool settings for Postgres.
type Config struct {
	Host     string
	Port     int
	DBName   string
	User     string
	Password string

	Secure    bool
	TLSConfig *tls.Config

	Timeout      time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	PingTimeout    time.Duration
	PingRetries    int
	PingBackoff    time.Duration
	PingBackoffMax time.Duration
}

// withDefaults applies sane defaults if fields are zero.
func (c *Config) withDefaults() *Config {
	if c.Port == 0 {
		c.Port = 5432
	}
	if c.Timeout == 0 {
		c.Timeout = 5 * time.Second
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 3 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 3 * time.Second
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 10
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 50
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = 30 * time.Minute
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 5 * time.Minute
	}
	if c.PingTimeout == 0 {
		c.PingTimeout = 2 * time.Second
	}
	if c.PingRetries == 0 {
		c.PingRetries = 3
	}
	if c.PingBackoff == 0 {
		c.PingBackoff = 300 * time.Millisecond
	}
	if c.PingBackoffMax == 0 {
		c.PingBackoffMax = 2 * time.Second
	}
	return c
}

// NewPostgresClient returns a ready-to-use bun.DB with health-checked connection.
func NewPostgresClient(cfg *Config) (*bun.DB, error) {
	cfg = cfg.withDefaults()

	if cfg.Host == "" || cfg.User == "" || cfg.Password == "" || cfg.DBName == "" {
		return nil, cerrors.ErrRequiredConnectionParamsAreEmpty
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	opts := []pgdriver.Option{
		pgdriver.WithNetwork("tcp"),
		pgdriver.WithAddr(addr),
		pgdriver.WithUser(cfg.User),
		pgdriver.WithPassword(cfg.Password),
		pgdriver.WithDatabase(cfg.DBName),
		pgdriver.WithTimeout(cfg.Timeout),
		pgdriver.WithReadTimeout(cfg.ReadTimeout),
		pgdriver.WithWriteTimeout(cfg.WriteTimeout),
	}

	if cfg.Secure {
		if cfg.TLSConfig != nil {
			opts = append(opts, pgdriver.WithTLSConfig(cfg.TLSConfig))
		} else {
			opts = append(opts, pgdriver.WithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
		}
	} else {
		opts = append(opts, pgdriver.WithInsecure(true))
	}

	conn := pgdriver.NewConnector(opts...)
	sqlDB := sql.OpenDB(conn)

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	db := bun.NewDB(sqlDB, pgdialect.New())

	backoff := cfg.PingBackoff
	for attempt := 0; attempt < cfg.PingRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.PingTimeout)
		err := db.PingContext(ctx)
		cancel()
		if err == nil {
			return db, nil
		}
		if attempt == cfg.PingRetries-1 {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("postgres ping failed after %d attempts: %w", cfg.PingRetries, err)
		}
		time.Sleep(backoff)
		backoff *= 2
		if backoff > cfg.PingBackoffMax {
			backoff = cfg.PingBackoffMax
		}
	}
	return db, nil
}

// ClosePostgres cleanly closes the underlying sql.DB.
func ClosePostgres(db *bun.DB) error {
	if db == nil {
		return nil
	}
	return db.DB.Close()
}
