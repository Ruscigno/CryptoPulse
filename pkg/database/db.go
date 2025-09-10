package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// DB wraps the database connection and provides additional functionality
type DB struct {
	*sqlx.DB
	logger *zap.Logger
	config config.Config
}

// Config holds database configuration
type Config struct {
	URL               string
	Host              string
	Port              int
	Name              string
	User              string
	Password          string
	SSLMode           string
	MaxOpenConns      int
	MaxIdleConns      int
	ConnMaxLifetime   time.Duration
	MigrationsPath    string
	ConnectionTimeout time.Duration
	QueryTimeout      time.Duration
}

// NewDB creates a new database connection
func NewDB(cfg config.Config, logger *zap.Logger) (*DB, error) {
	dbConfig := parseDBConfig(cfg)

	// Create connection string if not provided
	var dsn string
	if dbConfig.URL != "" {
		dsn = dbConfig.URL
	} else {
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			dbConfig.Host, dbConfig.Port, dbConfig.User, dbConfig.Password, dbConfig.Name, dbConfig.SSLMode)
	}

	logger.Info("Connecting to database",
		zap.String("host", dbConfig.Host),
		zap.Int("port", dbConfig.Port),
		zap.String("database", dbConfig.Name),
		zap.String("user", dbConfig.User))

	// Create connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), dbConfig.ConnectionTimeout)
	defer cancel()

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test connection
	if err := sqlDB.PingContext(ctx); err != nil {
		if closeErr := sqlDB.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to ping database: %w, and failed to close connection: %w", err, closeErr)
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConns)
	sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)

	// Create sqlx wrapper
	db := sqlx.NewDb(sqlDB, "postgres")

	logger.Info("Successfully connected to database")

	return &DB{
		DB:     db,
		logger: logger,
		config: cfg,
	}, nil
}

// parseDBConfig extracts database configuration from the main config
func parseDBConfig(cfg config.Config) Config {
	return Config{
		URL:               getEnvString("DATABASE_URL", ""),
		Host:              getEnvString("DATABASE_HOST", "localhost"),
		Port:              getEnvInt("DATABASE_PORT", 5432),
		Name:              getEnvString("DATABASE_NAME", "cryptopulse"),
		User:              getEnvString("DATABASE_USER", "cryptopulse"),
		Password:          getEnvString("DATABASE_PASSWORD", "cryptopulse_dev"),
		SSLMode:           getEnvString("DATABASE_SSL_MODE", "disable"),
		MaxOpenConns:      getEnvInt("DATABASE_MAX_OPEN_CONNS", 25),
		MaxIdleConns:      getEnvInt("DATABASE_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime:   getEnvDuration("DATABASE_CONN_MAX_LIFETIME", 5*time.Minute),
		MigrationsPath:    "file://pkg/database/migrations",
		ConnectionTimeout: 30 * time.Second,
		QueryTimeout:      30 * time.Second,
	}
}

// Close closes the database connection
func (db *DB) Close() error {
	db.logger.Info("Closing database connection")
	return db.DB.Close()
}

// Health checks the database connection health
func (db *DB) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	// Test with a simple query
	var result int
	if err := db.GetContext(ctx, &result, "SELECT 1"); err != nil {
		return fmt.Errorf("database query test failed: %w", err)
	}

	return nil
}

// RunMigrations runs database migrations
func (db *DB) RunMigrations() error {
	db.logger.Info("Running database migrations")

	driver, err := postgres.WithInstance(db.DB.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	dbConfig := parseDBConfig(db.config)
	m, err := migrate.NewWithDatabaseInstance(
		dbConfig.MigrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		db.logger.Warn("Could not get migration version", zap.Error(err))
	} else {
		db.logger.Info("Migration completed",
			zap.Uint("version", version),
			zap.Bool("dirty", dirty))
	}

	return nil
}

// RollbackMigrations rolls back database migrations
func (db *DB) RollbackMigrations(steps int) error {
	db.logger.Info("Rolling back database migrations", zap.Int("steps", steps))

	driver, err := postgres.WithInstance(db.DB.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	dbConfig := parseDBConfig(db.config)
	m, err := migrate.NewWithDatabaseInstance(
		dbConfig.MigrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Steps(-steps); err != nil {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		db.logger.Warn("Could not get migration version", zap.Error(err))
	} else {
		db.logger.Info("Migration rollback completed",
			zap.Uint("version", version),
			zap.Bool("dirty", dirty))
	}

	return nil
}

// BeginTx starts a new transaction
func (db *DB) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return db.BeginTxx(ctx, nil)
}

// WithTransaction executes a function within a database transaction
func (db *DB) WithTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				db.logger.Error("Failed to rollback transaction during panic", zap.Error(rbErr))
			}
			panic(p)
		} else if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				db.logger.Error("Failed to rollback transaction", zap.Error(rbErr))
			}
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				db.logger.Error("Failed to commit transaction", zap.Error(commitErr))
				err = commitErr
			}
		}
	}()

	err = fn(tx)
	return err
}

// GetStats returns database connection statistics
func (db *DB) GetStats() sql.DBStats {
	return db.DB.Stats()
}

// Helper functions for environment variable parsing
func getEnvString(key, defaultValue string) string {
	// This would typically use os.Getenv, but for now return default
	// In a real implementation, you'd integrate with your config system
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	// This would typically parse from os.Getenv, but for now return default
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	// This would typically parse from os.Getenv, but for now return default
	return defaultValue
}
