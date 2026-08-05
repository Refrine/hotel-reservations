package integration

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)

func TestIndexes(t *testing.T) {
    ctx := context.Background()

    // Запуск PostgreSQL
    postgresContainer, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:16-alpine"),
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database is ready").
                WithOccurrence(2).
                WithStartupTimeout(5*time.Second),
        ),
    )
    require.NoError(t, err)
    defer postgresContainer.Terminate(ctx)

    dsn, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)


    db, err := sql.Open("postgres", dsn)
    require.NoError(t, err)
    defer db.Close()

    goose.SetDialect("postgres")
    err = goose.Up(db, "../migrations")
    require.NoError(t, err)

    t.Run("check bookings indexes", func(t *testing.T) {
        indexes := getTableIndexes(t, db, "bookings")
        
        required := []string{
            "idx_bookings_catalog_request_id",
            "idx_bookings_created_at",
            "idx_bookings_created_status",
            "idx_bookings_created_resource",
            "idx_bookings_status",
            "idx_bookings_user_id",
            "idx_bookings_resource_id",
            "idx_bookings_id_desc",
        }

        for _, idx := range required {
            assert.Contains(t, indexes, idx, "индекс %s должен существовать", idx)
        }
    })

    t.Run("check outbox indexes", func(t *testing.T) {
        indexes := getTableIndexes(t, db, "outbox_messages")
        
        required := []string{
            "idx_outbox_status_created",
            "idx_outbox_status_retry",
        }

        for _, idx := range required {
            assert.Contains(t, indexes, idx, "индекс %s должен существовать", idx)
        }
    })

    t.Run("check booking_history indexes", func(t *testing.T) {
        indexes := getTableIndexes(t, db, "booking_history")
        
        assert.Contains(t, indexes, "idx_booking_history_booking_created")
    })
}

func TestExplainQueries(t *testing.T) {
    ctx := context.Background()

    postgresContainer, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:16-alpine"),
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2),
        ),
    )
    require.NoError(t, err)
    defer postgresContainer.Terminate(ctx)

    dsn, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    db, err := sql.Open("postgres", dsn)
    require.NoError(t, err)
    defer db.Close()

    goose.Up(db, "../migrations")

   
    testCases := []struct {
        name string
        sql  string
    }{
        {
            name: "catalog_request_id lookup",
            sql:  "EXPLAIN SELECT * FROM bookings WHERE catalog_request_id = 'test-123'",
        },
        {
            name: "statistics date range",
            sql:  "EXPLAIN SELECT COUNT(*) FROM bookings WHERE created_at >= '2024-01-01' AND created_at < '2024-02-01'",
        },
        {
            name: "outbox pending messages",
            sql:  "EXPLAIN SELECT * FROM outbox_messages WHERE status = 'pending' ORDER BY created_at LIMIT 50",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            var plan string
            err := db.QueryRow(tc.sql).Scan(&plan)
            require.NoError(t, err)
            
            assert.Contains(t, plan, "Index", 
                "Запрос должен использовать индекс:\n%s", plan)
        })
    }
}

func getTableIndexes(t *testing.T, db *sql.DB, tableName string) []string {
    rows, err := db.Query(`
        SELECT indexname 
        FROM pg_indexes 
        WHERE tablename = $1 
        ORDER BY indexname
    `, tableName)
    require.NoError(t, err)
    defer rows.Close()

    var indexes []string
    for rows.Next() {
        var name string
        err := rows.Scan(&name)
        require.NoError(t, err)
        indexes = append(indexes, name)
    }

    return indexes
}