package testhelper

import (
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func NewTestPgxConn(t *testing.T) *pgx.Conn {
	t.Helper()

	ctx := t.Context()

	connString := os.Getenv("DATABASE_URL")

	if connString == "" {
		t.Skipf("skipping due to missing environment variable %v", "DATABASE_URL")
	}

	config, err := pgx.ParseConfig(connString)
	require.NoError(t, err)

	conn, err := pgx.ConnectConfig(ctx, config)
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close(ctx)
	})

	return conn
}
