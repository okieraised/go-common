package database

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/okieraised/go-common/config"
	"github.com/okieraised/go-common/constants"
	"github.com/okieraised/go-common/infrastructures/logging"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type TLSConfig struct {
	EnableTLS       bool
	CAFile          string
	ClientCertFile  string
	ClientKeyFile   string
	InsecureSkipTLS bool
}

type RetryConfig struct {
	MaxRetries int
	Interval   time.Duration
	MaxBackoff time.Duration
}

func BuildTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	if cfg.CAFile != "" {
		caPem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPem) {
			return nil, errors.New("failed to load CA certificate")
		}
		tlsConfig.RootCAs = caPool
	}

	if cfg.ClientCertFile != "" && cfg.ClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	tlsConfig.InsecureSkipVerify = cfg.InsecureSkipTLS
	return tlsConfig, nil
}

func retryConnect(db *bun.DB, cfg *RetryConfig) error {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 5
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 2 * time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 30 * time.Second
	}

	backoff := cfg.Interval

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := db.PingContext(ctx)
		cancel()

		if err == nil {
			return nil // success
		}

		logging.GetDefault().Info(fmt.Sprintf("Postgres connection failed (attempt %d/%d): %v", attempt, cfg.MaxRetries, err))
		if attempt == cfg.MaxRetries {
			return err
		}

		time.Sleep(backoff)
		backoff *= 2
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}

	return errors.New("unreachable: retry logic failed")
}

func NewPostgresClient(
	host, port, dbname, userName, password string,
	tlsCfg *TLSConfig,
	retryCfg *RetryConfig,
) (*bun.DB, error) {

	logger := logging.GetDefault()
	logger.Info("Initializing postgres client...")

	if host == "" || userName == "" || password == "" || dbname == "" {
		return nil, errors.New("postgres init error: missing required fields")
	}

	getDuration := func(key string, fallback time.Duration) time.Duration {
		if v := viper.GetDuration(key); v > 0 {
			return v
		}
		return fallback
	}

	getInt := func(key string, fallback int) int {
		if v := viper.GetInt(key); v > 0 {
			return v
		}
		return fallback
	}

	dbTimeout := getDuration(config.DatabaseTimeout, constants.DefaultDBTimeout)
	dbReadTimeout := getDuration(config.DatabaseReadTimeout, constants.DefaultDBReadTimeout)
	dbWriteTimeout := getDuration(config.DatabaseWriteTimeout, constants.DefaultDBWriteTimeout)
	dbMaxIdleConn := getInt(config.DatabaseMaxIdleConn, constants.DefaultDBMaxIdleConn)
	dbMaxOpenConn := getInt(config.DatabaseMaxOpenConn, constants.DefaultDBMaxOpenConn)
	dbConnMaxLifetime := getDuration(config.DatabaseConnMaxLifetime, constants.DefaultDBConnMaxLifetime)
	dbConnMaxIdleTime := getDuration(config.DatabaseConnMaxIdleTime, constants.DefaultDBConnMaxIdleTime)

	connectorOpts := []pgdriver.Option{
		pgdriver.WithNetwork("tcp"),
		pgdriver.WithAddr(fmt.Sprintf("%s:%s", host, port)),
		pgdriver.WithUser(userName),
		pgdriver.WithPassword(password),
		pgdriver.WithDatabase(dbname),
		pgdriver.WithTimeout(dbTimeout),
		pgdriver.WithReadTimeout(dbReadTimeout),
		pgdriver.WithWriteTimeout(dbWriteTimeout),
	}

	var tlsConfig *tls.Config
	var err error

	if tlsCfg != nil {
		if tlsCfg.EnableTLS {
			tlsConfig, err = BuildTLSConfig(tlsCfg)
			if err != nil {
				return nil, fmt.Errorf("failed to build TLS config: %w", err)
			}
		}
		if tlsCfg.EnableTLS {
			connectorOpts = append(connectorOpts, pgdriver.WithTLSConfig(tlsConfig))
		} else {
			connectorOpts = append(connectorOpts, pgdriver.WithInsecure(true))
		}
	}

	pgConnector := pgdriver.NewConnector(connectorOpts...)
	sqlDB := sql.OpenDB(pgConnector)

	sqlDB.SetMaxIdleConns(dbMaxIdleConn)
	sqlDB.SetMaxOpenConns(dbMaxOpenConn)
	sqlDB.SetConnMaxIdleTime(dbConnMaxIdleTime)
	sqlDB.SetConnMaxLifetime(dbConnMaxLifetime)

	db := bun.NewDB(sqlDB, pgdialect.New())

	if retryCfg != nil {
		if err := retryConnect(db, retryCfg); err != nil {
			return nil, fmt.Errorf("failed to connect to postgres after retries: %w", err)
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := db.PingContext(ctx)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("failed to connect to postgres: %w", err)
		}
	}

	logger.Info("postgres client initialized successfully")
	return db, nil
}
