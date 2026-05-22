package cmdutil

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/adjust/rmq/v5"
	"github.com/go-redis/redis/extra/redisotel/v8"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sideshow/apns2/token"

	"go.uber.org/zap"
)

func NewLogger(service string) *zap.Logger {
	env := os.Getenv("ENV")
	logger, _ := zap.NewProduction(zap.Fields(
		zap.String("env", env),
		zap.String("service", service),
	))

	if env == "" || env == "development" {
		logger, _ = zap.NewDevelopment()
	}

	return logger
}

// LoadAPNS reads the four APNs env vars (APPLE_KEY_PATH, APPLE_KEY_ID,
// APPLE_TEAM_ID, APPLE_APNS_TOPIC) and returns the signing token plus topic.
// Returns a descriptive error naming the missing var or unreadable file so
// startup failures show up as a single log line instead of a panic stack.
func LoadAPNS() (*token.Token, string, error) {
	keyPath := os.Getenv("APPLE_KEY_PATH")
	if keyPath == "" {
		return nil, "", fmt.Errorf("APPLE_KEY_PATH env var is required (path to the APNs .p8 auth key)")
	}

	authKey, err := token.AuthKeyFromFile(keyPath)
	if err != nil {
		return nil, "", fmt.Errorf("loading APNs auth key from APPLE_KEY_PATH (%s): %w", keyPath, err)
	}

	keyID := os.Getenv("APPLE_KEY_ID")
	if keyID == "" {
		return nil, "", fmt.Errorf("APPLE_KEY_ID env var is required (APNs key ID from developer.apple.com)")
	}

	teamID := os.Getenv("APPLE_TEAM_ID")
	if teamID == "" {
		return nil, "", fmt.Errorf("APPLE_TEAM_ID env var is required (Apple Developer team ID)")
	}

	topic := os.Getenv("APPLE_APNS_TOPIC")
	if topic == "" {
		return nil, "", fmt.Errorf("APPLE_APNS_TOPIC env var is required (bundle ID of the sideloaded Apollo build)")
	}

	return &token.Token{
		AuthKey: authKey,
		KeyID:   keyID,
		TeamID:  teamID,
	}, topic, nil
}

func NewStatsdClient(tags ...string) (statsd.ClientInterface, error) {
	url := os.Getenv("STATSD_URL")
	if url == "" {
		return &statsd.NoOpClient{}, nil
	}

	if env := os.Getenv("ENV"); env != "" {
		tags = append(tags, fmt.Sprintf("env:%s", env))
	}

	return statsd.New(url, statsd.WithTags(tags))
}

func NewRedisLocksClient(ctx context.Context, maxConns int) (*redis.Client, error) {
	return newRedisClient(ctx, "REDIS_LOCKS_URL", maxConns)
}

func NewRedisQueueClient(ctx context.Context, maxConns int) (*redis.Client, error) {
	return newRedisClient(ctx, "REDIS_QUEUE_URL", maxConns)
}

func newRedisClient(ctx context.Context, env string, maxConns int) (*redis.Client, error) {
	opt, err := redis.ParseURL(os.Getenv(env))
	if err != nil {
		return nil, err
	}
	opt.PoolSize = maxConns

	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	client.AddHook(redisotel.NewTracingHook())

	return client, nil
}

func NewDatabasePool(ctx context.Context, maxConns int) (*pgxpool.Pool, error) {
	if maxConns == 0 {
		maxConns = 1
	}

	url := fmt.Sprintf(
		"%s?pool_max_conns=%d&pool_min_conns=%d",
		os.Getenv("DATABASE_CONNECTION_POOL_URL"),
		maxConns,
		2,
	)
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}

	// Setting the build statement cache to nil helps this work with pgbouncer
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Second
	return pgxpool.NewWithConfig(ctx, config)
}

func NewQueueClient(logger *zap.Logger, conn *redis.Client, identifier string) (rmq.Connection, error) {
	errChan := make(chan error, 10)
	go func() {
		for err := range errChan {
			logger.Error("error occurred within queue", zap.Error(err))
		}
	}()

	return rmq.OpenConnectionWithRedisClient(identifier, conn, errChan)
}
