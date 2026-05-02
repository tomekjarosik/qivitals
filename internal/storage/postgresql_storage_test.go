package storage

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresStorageContract(t *testing.T) {
	ctx := context.Background()

	// 1. Spin up a temporary Postgres container using Testcontainers
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("onestatus"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10*time.Second),
		),
	)
	require.NoError(t, err, "Failed to start postgres container")

	// Ensure container is destroyed after all contract tests finish
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// 2. Connect pgxpool
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	// 3. Initialize our storage and run schema migrations
	pgStorage := NewPostgresSensorStorage(pool)
	err = pgStorage.InitSchema(ctx)
	require.NoError(t, err, "Failed to initialize database schema")

	// 4. Define setup & teardown for each individual contract sub-test
	setup := func() SensorStorage {
		// Truncate the table so every test starts with a completely empty database
		_, err := pool.Exec(ctx, "TRUNCATE TABLE sensors")
		require.NoError(t, err)
		return pgStorage
	}

	teardown := func() {
		// Nothing specific to tear down per test, truncation handles it
	}

	// 5. Run the exact same test suite we used for MemoryStorage!
	RunStorageContractTests(t, setup, teardown)
	RunExtendedStorageContractTests(t, setup, teardown)
}
