package repository_test

import (
	"testing"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/postgres"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/repository"
)

func TestRepositoryInterfaceImplementations(t *testing.T) {
	var _ repository.AccountRepository = postgres.NewPostgresAccountRepository(nil, nil)
	var _ repository.IdempotencyRepository = postgres.NewPostgresIdempotencyRepository(nil, nil)
	var _ repository.LedgerRepository = postgres.NewPostgresLedgerRepository()
}
