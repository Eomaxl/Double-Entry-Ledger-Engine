package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/postgres"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GetAccountStatement retrieves transactions for an account in chronological order
func (q *PostgresQueryService) GetAccountStatement(ctx context.Context, accountID string, filter StatementFilter) (*Statement, error) {
	q.logger.Debug("getting account statement",
		zap.String("account_id", accountID),
		zap.Any("filter", filter))

	accountUUID, err := uuid.Parse(accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %s", accountID)
	}

	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	baseQuery := `
		SELECT DISTINCT t.transaction_id, t.idempotency_key, t.state, t.posted_at, t.settled_at, 
		       t.reversed_by_transaction_id, t.reverses_transaction_id, t.metadata
		FROM transactions t
		INNER JOIN entries e ON t.transaction_id = e.transaction_id
		WHERE e.account_id = $1
	`

	countQuery := `
		SELECT COUNT(DISTINCT t.transaction_id)
		FROM transactions t
		INNER JOIN entries e ON t.transaction_id = e.transaction_id
		WHERE e.account_id = $1
	`

	var whereClauses []string
	var args []interface{}
	args = append(args, accountUUID)
	argIndex := 2

	if filter.Currency != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("e.currency_code = $%d", argIndex))
		args = append(args, *filter.Currency)
		argIndex++
	}

	if filter.StartDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.posted_at >= $%d", argIndex))
		args = append(args, *filter.StartDate)
		argIndex++
	}

	if filter.EndDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.posted_at <= $%d", argIndex))
		args = append(args, *filter.EndDate)
		argIndex++
	}

	if len(whereClauses) > 0 {
		whereClause := " AND " + strings.Join(whereClauses, " AND ")
		baseQuery += whereClause
		countQuery += whereClause
	}

	orderClause := " ORDER BY t.posted_at ASC"

	fullQuery := baseQuery + orderClause +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)

	queryArgs := append(args, filter.Limit, filter.Offset)

	var totalCount int64
	err = q.pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		q.logger.Error("failed to count account transactions", zap.Error(err), zap.String("account_id", accountID))
		return nil, fmt.Errorf("failed to count account transactions: %w", err)
	}

	rows, err := q.pool.Query(ctx, fullQuery, queryArgs...)
	if err != nil {
		q.logger.Error("failed to query account transactions", zap.Error(err), zap.String("account_id", accountID))
		return nil, fmt.Errorf("failed to query account transactions: %w", err)
	}
	defer rows.Close()

	transactions := make([]domain.Transaction, 0)
	transactionIDs := make([]uuid.UUID, 0)

	for rows.Next() {
		txn, err := postgres.ScanTransactionFromRow(rows)
		if err != nil {
			q.logger.Error("failed to scan transaction row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan transaction row: %w", err)
		}

		transactions = append(transactions, *txn)

		txnUUID, err := uuid.Parse(txn.TransactionID)
		if err != nil {
			q.logger.Error("failed to parse transaction ID", zap.Error(err), zap.String("transaction_id", txn.TransactionID))
			return nil, fmt.Errorf("failed to parse transaction ID: %w", err)
		}
		transactionIDs = append(transactionIDs, txnUUID)
	}

	if err := rows.Err(); err != nil {
		q.logger.Error("error iterating transaction rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating transaction rows: %w", err)
	}

	if len(transactionIDs) > 0 {
		entriesMap, err := q.ledger.FetchEntriesByTransactionIDs(ctx, q.pool, transactionIDs)
		if err != nil {
			return nil, err
		}

		for i := range transactions {
			txnUUID, _ := uuid.Parse(transactions[i].TransactionID)
			transactions[i].Entries = entriesMap[txnUUID]
		}
	}

	hasMore := int64(filter.Offset+len(transactions)) < totalCount

	result := &Statement{
		AccountID:    accountID,
		Currency:     filter.Currency,
		Transactions: transactions,
		TotalCount:   totalCount,
		Limit:        filter.Limit,
		Offset:       filter.Offset,
		HasMore:      hasMore,
	}

	q.logger.Debug("account statement retrieved successfully",
		zap.String("account_id", accountID),
		zap.Int("count", len(transactions)),
		zap.Int64("total_count", totalCount),
		zap.Bool("has_more", hasMore))

	return result, nil
}
