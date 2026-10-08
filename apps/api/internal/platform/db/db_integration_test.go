//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/db"
	"lms/api/internal/platform/testdb"
)

// testDatabaseURL giữ khóa database test qua testdb.Open để không chạy song song với package tích hợp khác.
func testDatabaseURL(t *testing.T) string {
	t.Helper()
	testdb.Open(t)
	return testdb.URL(t)
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestOpenPing(t *testing.T) {
	ctx := testContext(t)
	dbx, err := db.Open(ctx, testDatabaseURL(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = dbx.Close() })

	var one int
	require.NoError(t, dbx.GetContext(ctx, &one, "SELECT 1"))
	assert.Equal(t, 1, one)
}

func TestOpenRejectsInvalidDSNWithoutLeakingIt(t *testing.T) {
	_, err := db.Open(testContext(t), "postgres://u:s3cr3t@%zz")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "s3cr3t")
}

func TestMigrateRoundTrip(t *testing.T) {
	ctx := testContext(t)
	url := testDatabaseURL(t)

	// Pool của ứng dụng phải sống sót qua mọi lần migrate (migrate dùng *sql.DB riêng).
	app, err := db.Open(ctx, url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })

	require.NoError(t, db.Migrate(ctx, url, db.Down, 0))
	_, _, err = db.Version(ctx, url)
	require.ErrorIs(t, err, db.ErrNilVersion)

	latest := testdb.LatestVersion(t)
	require.Positive(t, latest)

	require.NoError(t, db.Migrate(ctx, url, db.Up, 0))
	v, dirty, err := db.Version(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, latest, v)
	assert.False(t, dirty)

	require.NoError(t, db.Migrate(ctx, url, db.Up, 0), "up lần hai không có gì để chạy, không phải lỗi")

	require.NoError(t, db.Migrate(ctx, url, db.Down, 1))
	v, _, err = db.Version(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, latest-1, v)

	require.NoError(t, db.Migrate(ctx, url, db.Up, 1))
	v, _, err = db.Version(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, latest, v)

	require.NoError(t, app.PingContext(ctx))
}

func TestMigrateRejectsInvalidInput(t *testing.T) {
	ctx := testContext(t)
	url := testDatabaseURL(t)
	require.Error(t, db.Migrate(ctx, url, "sideways", 0))
	require.Error(t, db.Migrate(ctx, url, db.Up, -1))
}
