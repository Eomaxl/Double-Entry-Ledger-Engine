package transaction

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type txStarter interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// SerializableTransaction executes a function within a SERIALIZABLE transaction
func (cc *ConcurrencyController) SerializableTransaction(ctx context.Context, starter txStarter, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := starter.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
	if err != nil {
		cc.logger.Error("failed to begin serializable transaction", zap.Error(err))
		return fmt.Errorf("failed to begin serializable transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		cc.logger.Error("failed to commit serializable transaction", zap.Error(err))
		return fmt.Errorf("failed to commit serializable transaction: %w", err)
	}

	return nil
}
