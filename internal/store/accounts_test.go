package store_test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"github.com/langgexyz/open-im-node-server/internal/store"
)

func testDB(t *testing.T) *store.Accounts {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not set, skipping MySQL integration test")
	}
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	accounts, err := store.NewAccounts(db)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Exec("TRUNCATE TABLE accounts")
		db.Close()
	})
	return accounts
}

func TestAccountsGetOrCreate(t *testing.T) {
	accounts := testDB(t)

	id1, err := accounts.GetOrCreate("app_uid_123")
	require.NoError(t, err)
	require.Greater(t, id1, uint64(0))

	id2, err := accounts.GetOrCreate("app_uid_123")
	require.NoError(t, err)
	require.Equal(t, id1, id2)

	id3, err := accounts.GetOrCreate("app_uid_456")
	require.NoError(t, err)
	require.NotEqual(t, id1, id3)
}

func TestAccountsGetAppUIDs(t *testing.T) {
	accounts := testDB(t)

	id1, _ := accounts.GetOrCreate("app_uid_aaa")
	id2, _ := accounts.GetOrCreate("app_uid_bbb")
	id3, _ := accounts.GetOrCreate("app_uid_ccc")

	result, err := accounts.GetAppUIDs([]uint64{id1, id3, 99999})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "app_uid_aaa", result[id1])
	require.Equal(t, "app_uid_ccc", result[id3])
	_ = id2
}
