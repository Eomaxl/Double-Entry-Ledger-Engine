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

// ListTransactions retrieves transactions with filtering and pagination
func (q *PostgresQueryService) ListTransactions(ctx context.Context, filter TransactionFilter) (*TransactionPage, error) {
	q.logger.Debug("listing transactions with filter",
		zap.Any("filter", filter))

	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	if filter.OrderBy == "" {
		filter.OrderBy = "timestamp_desc"
	}

	baseQuery := `
		SELECT DISTINCT t.transaction_id, t.idempotency_key, t.state, t.posted_at, t.settled_at, 
		       t.reversed_by_transaction_id, t.reverses_transaction_id, t.metadata
		FROM transactions t
	`

	countQuery := `
		SELECT COUNT(DISTINCT t.transaction_id)
		FROM transactions t
	`

	var joinClauses []string
	var whereClauses []string
	var args []interface{}
	argIndex := 1

	if filter.AccountID != nil || filter.Currency != nil {
		joinClauses = append(joinClauses, "INNER JOIN entries e ON t.transaction_id = e.transaction_id")
	}

	if filter.AccountID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("e.account_id = $%d", argIndex))
		args = append(args, *filter.AccountID)
		argIndex++
	}

	if filter.Currency != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("e.currency_code = $%d", argIndex))
		args = append(args, *filter.Currency)
		argIndex++
	}

	if filter.State != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.state = $%d", argIndex))
		args = append(args, *filter.State)
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

	for key, value := range filter.Metadata {
		whereClauses = append(whereClauses, fmt.Sprintf("t.metadata ->> $%d = $%d", argIndex, argIndex+1))
		args = append(args, key, value)
		argIndex += 2
	}

	joinClause := strings.Join(joinClauses, " ")

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	orderClause := "ORDER BY t.posted_at DESC"
	if filter.OrderBy == "timestamp_asc" {
		orderClause = "ORDER BY t.posted_at ASC"
	}

	fullQuery := baseQuery + joinClause + " " + whereClause + " " + orderClause +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	fullCountQuery := countQuery + joinClause + " " + whereClause

	queryArgs := append(args, filter.Limit, filter.Offset)

	var totalCount int64
	err := q.pool.QueryRow(ctx, fullCountQuery, args...).Scan(&totalCount)
	if err != nil {
		q.logger.Error("failed to count transactions", zap.Error(err))
		return nil, fmt.Errorf("failed to count transactions: %w", err)
	}

	rows, err := q.pool.Query(ctx, fullQuery, queryArgs...)
	if err != nil {
		q.logger.Error("failed to query transactions", zap.Error(err))
		return nil, fmt.Errorf("failed to query transactions: %w", err)
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

	result := &TransactionPage{
		Transactions: transactions,
		TotalCount:   totalCount,
		Limit:        filter.Limit,
		Offset:       filter.Offset,
		HasMore:      hasMore,
	}

	q.logger.Debug("transactions listed successfully",
		zap.Int("count", len(transactions)),
		zap.Int64("total_count", totalCount),
		zap.Bool("has_more", hasMore))

	return result, nil
}
