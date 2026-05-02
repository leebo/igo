package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type databaseTestUser struct {
	ID        int64 `gorm:"primaryKey"`
	Name      string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func TestOpenUnsupportedDialectAndMustOpen(t *testing.T) {
	db, err := Open(Config{Dialect: "oracle", DSN: "unused"})
	assert.Nil(t, db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported dialect")

	assert.Panics(t, func() {
		MustOpen(Config{Dialect: "oracle", DSN: "unused"})
	})
}

func TestBaseRepositoryIntegrationMySQL(t *testing.T) {
	dsn := os.Getenv("IGO_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("IGO_TEST_MYSQL_DSN is not set")
	}
	testBaseRepositoryIntegration(t, Config{Dialect: "mysql", DSN: dsn})
}

func TestBaseRepositoryIntegrationPostgres(t *testing.T) {
	dsn := os.Getenv("IGO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("IGO_TEST_POSTGRES_DSN is not set")
	}
	testBaseRepositoryIntegration(t, Config{Dialect: "postgres", DSN: dsn})
}

func testBaseRepositoryIntegration(t *testing.T, cfg Config) {
	t.Helper()

	db, err := Open(cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })

	table := fmt.Sprintf("igo_repository_test_%d", time.Now().UnixNano())
	require.NoError(t, db.Table(table).AutoMigrate(&databaseTestUser{}))
	t.Cleanup(func() { assert.NoError(t, db.Exec("DROP TABLE "+table).Error) })

	repo := NewRepository[databaseTestUser](db, table)
	assert.NotNil(t, repo)
	assert.Implements(t, (*Repository[databaseTestUser])(nil), repo)

	ctx := context.Background()
	alice := &databaseTestUser{Name: "Alice", Email: "alice@example.com"}
	bob := &databaseTestUser{Name: "Bob", Email: "bob@example.com"}

	require.NoError(t, repo.Create(ctx, alice))
	require.NoError(t, repo.Create(ctx, bob))
	assert.NotZero(t, alice.ID)

	got, err := repo.GetByID(ctx, alice.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alice", got.Name)

	got.Name = "Alice Updated"
	require.NoError(t, repo.Update(ctx, got))

	found, err := repo.FindOne(ctx, "email = ?", "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, "Alice Updated", found.Name)

	all, err := repo.Find(ctx, "email LIKE ?", "%@example.com")
	require.NoError(t, err)
	assert.Len(t, all, 2)

	page, total, err := repo.List(ctx, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, page, 1)

	require.NoError(t, repo.Delete(ctx, alice.ID))
	_, err = repo.GetByID(ctx, alice.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
