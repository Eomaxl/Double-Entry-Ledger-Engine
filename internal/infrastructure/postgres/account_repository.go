package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var _ repository.AccountRepository = (*PostgresAccountRepository)(nil)

type PostgresAccountRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewPostgresAccountRepository(pool *pgxpool.Pool, logger *zap.Logger) *PostgresAccountRepository {
	return &PostgresAccountRepository{
		pool:   pool,
		logger: logger,
	}
}

// CreateAccount creates a new account with the specified currencies
func (r *PostgresAccountRepository) CreateAccount(ctx context.Context, req domain.CreateAccountRequest) (*domain.Account, error) {
	// Validate account type
	if !req.AccountType.IsValid() {
		return nil, fmt.Errorf("invalid account type: %s", req.AccountType)
	}

	// Validate at least one currency is provided
	if len(req.Currencies) == 0 {
		return nil, fmt.Errorf("at least one currency must be specified")
	}

	// Start a transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("failed to begin transaction", zap.Error(err))
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Generate UUID for the account
	accountID := uuid.New()
	now := time.Now()

	// Marshal metadata to JSON
	var metadataJSON []byte
	if req.Metadata != nil {
		metadataJSON, err = json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	// Insert account
	insertAccountSQL := `
		INSERT INTO accounts (account_id, account_type, created_at, updated_at, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = tx.Exec(ctx, insertAccountSQL, accountID, req.AccountType, now, now, metadataJSON)
	if err != nil {
		r.logger.Error("failed to insert account", zap.Error(err), zap.String("account_id", accountID.String()))
		return nil, fmt.Errorf("failed to insert account: %w", err)
	}

	// Insert account currencies
	insertCurrencySQL := `
		INSERT INTO account_currencies (account_id, currency_code, allow_negative, created_at)
		VALUES ($1, $2, $3, $4)
	`

	currencies := make([]domain.AccountCurrency, 0, len(req.Currencies))
	for _, currencyConfig := range req.Currencies {
		_, err = tx.Exec(ctx, insertCurrencySQL, accountID, currencyConfig.CurrencyCode, currencyConfig.AllowNegative, now)
		if err != nil {
			r.logger.Error("failed to insert account currency",
				zap.Error(err),
				zap.String("account_id", accountID.String()),
				zap.String("currency", currencyConfig.CurrencyCode))
			return nil, fmt.Errorf("failed to insert account currency: %w", err)
		}

		currencies = append(currencies, domain.AccountCurrency{
			CurrencyCode:  currencyConfig.CurrencyCode,
			AllowNegative: currencyConfig.AllowNegative,
			CreatedAt:     now,
		})
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		r.logger.Error("failed to commit transaction", zap.Error(err))
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("account created",
		zap.String("account_id", accountID.String()),
		zap.String("account_type", string(req.AccountType)),
		zap.Int("currency_count", len(currencies)))

	return &domain.Account{
		AccountID:   accountID,
		AccountType: req.AccountType,
		Currencies:  currencies,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    req.Metadata,
	}, nil
}

type pgQueryable interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// GetAccount retrieves an account by ID with its currency associations
func (r *PostgresAccountRepository) GetAccount(ctx context.Context, accountID uuid.UUID) (*domain.Account, error) {
	return r.getAccount(ctx, r.pool, accountID)
}

// GetAccountInTx loads the account using an open transaction (same snapshot as other tx operations).
func (r *PostgresAccountRepository) GetAccountInTx(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) (*domain.Account, error) {
	return r.getAccount(ctx, tx, accountID)
}

func (r *PostgresAccountRepository) getAccount(ctx context.Context, q pgQueryable, accountID uuid.UUID) (*domain.Account, error) {
	queryAccountSQL := `
		SELECT account_id, account_type, created_at, updated_at, metadata
		FROM accounts
		WHERE account_id = $1
	`

	var account domain.Account
	var metadataJSON []byte

	err := q.QueryRow(ctx, queryAccountSQL, accountID).Scan(
		&account.AccountID,
		&account.AccountType,
		&account.CreatedAt,
		&account.UpdatedAt,
		&metadataJSON,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("account not found: %s", accountID)
		}
		r.logger.Error("failed to query account", zap.Error(err), zap.String("account_id", accountID.String()))
		return nil, fmt.Errorf("failed to query account: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &account.Metadata); err != nil {
			r.logger.Error("failed to unmarshal metadata", zap.Error(err))
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	queryCurrenciesSQL := `
		SELECT currency_code, allow_negative, created_at
		FROM account_currencies
		WHERE account_id = $1
		ORDER BY currency_code
	`

	rows, err := q.Query(ctx, queryCurrenciesSQL, accountID)
	if err != nil {
		r.logger.Error("failed to query account currencies", zap.Error(err), zap.String("account_id", accountID.String()))
		return nil, fmt.Errorf("failed to query account currencies: %w", err)
	}
	defer rows.Close()

	currencies := make([]domain.AccountCurrency, 0)
	for rows.Next() {
		var currency domain.AccountCurrency
		if err := rows.Scan(&currency.CurrencyCode, &currency.AllowNegative, &currency.CreatedAt); err != nil {
			r.logger.Error("failed to scan currency row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan currency row: %w", err)
		}
		currencies = append(currencies, currency)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("error iterating currency rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating currency rows: %w", err)
	}

	account.Currencies = currencies

	return &account, nil
}

// ListAccounts retrieves accounts with optional filtering
func (r *PostgresAccountRepository) ListAccounts(ctx context.Context, filter domain.AccountFilter) ([]domain.Account, error) {
	// Build query with filters
	query := `
		SELECT DISTINCT a.account_id, a.account_type, a.created_at, a.updated_at, a.metadata
		FROM accounts a
	`

	args := make([]interface{}, 0)
	argIndex := 1
	whereClauses := make([]string, 0)

	// Add currency filter if specified
	if filter.CurrencyCode != nil {
		query += ` INNER JOIN account_currencies ac ON a.account_id = ac.account_id`
		whereClauses = append(whereClauses, fmt.Sprintf(" ac.currency_code = $%d", argIndex))
		args = append(args, *filter.CurrencyCode)
		argIndex++
	}

	// Add account type filter if specified
	if filter.AccountType != nil {
		whereClauses = append(whereClauses, fmt.Sprintf(" a.account_type = $%d", argIndex))
		args = append(args, *filter.AccountType)
		argIndex++
	}

	// Add WHERE clause if filters exist
	if len(whereClauses) > 0 {
		query += " WHERE" + whereClauses[0]
		for i := 1; i < len(whereClauses); i++ {
			query += " AND" + whereClauses[i]
		}
	}

	// Add ordering
	query += ` ORDER BY a.created_at DESC`

	// Add pagination
	limit := 100 // default limit
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	query += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, limit)
	argIndex++

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
	}

	// Execute query
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("failed to query accounts", zap.Error(err))
		return nil, fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]domain.Account, 0)
	accountIDs := make([]uuid.UUID, 0)

	for rows.Next() {
		var account domain.Account
		var metadataJSON []byte

		if err := rows.Scan(&account.AccountID, &account.AccountType, &account.CreatedAt, &account.UpdatedAt, &metadataJSON); err != nil {
			r.logger.Error("failed to scan account row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan account row: %w", err)
		}

		// Unmarshal metadata
		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &account.Metadata); err != nil {
				r.logger.Error("failed to unmarshal metadata", zap.Error(err))
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		accounts = append(accounts, account)
		accountIDs = append(accountIDs, account.AccountID)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("error iterating account rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating account rows: %w", err)
	}

	// If no accounts found, return empty slice
	if len(accounts) == 0 {
		return accounts, nil
	}

	// Fetch currencies for all accounts in a single query
	currenciesMap, err := r.fetchCurrenciesForAccounts(ctx, accountIDs)
	if err != nil {
		return nil, err
	}

	// Attach currencies to accounts
	for i := range accounts {
		accounts[i].Currencies = currenciesMap[accounts[i].AccountID]
	}

	return accounts, nil
}

func (r *PostgresAccountRepository) fetchCurrenciesForAccounts(ctx context.Context, accountIDs []uuid.UUID) (map[uuid.UUID][]domain.AccountCurrency, error) {
	query := `
		SELECT account_id, currency_code, allow_negative, created_at
		FROM account_currencies
		WHERE account_id = ANY($1)
		ORDER BY account_id, currency_code
	`

	rows, err := r.pool.Query(ctx, query, accountIDs)
	if err != nil {
		r.logger.Error("failed to query currencies for accounts", zap.Error(err))
		return nil, fmt.Errorf("failed to query currencies for accounts: %w", err)
	}
	defer rows.Close()

	currenciesMap := make(map[uuid.UUID][]domain.AccountCurrency)

	for rows.Next() {
		var accountID uuid.UUID
		var currency domain.AccountCurrency

		if err := rows.Scan(&accountID, &currency.CurrencyCode, &currency.AllowNegative, &currency.CreatedAt); err != nil {
			r.logger.Error("failed to scan currency row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan currency row: %w", err)
		}

		currenciesMap[accountID] = append(currenciesMap[accountID], currency)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("error iterating currency rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating currency rows: %w", err)
	}

	return currenciesMap, nil
}

func (r *PostgresAccountRepository) UpdateAccountMetadata(ctx context.Context, accountID uuid.UUID, metadata map[string]interface{}) error {
	// Marshal metadata to JSON
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Update account metadata
	updateSQL := `
		UPDATE accounts
		SET metadata = $1, updated_at = $2
		WHERE account_id = $3
	`

	result, err := r.pool.Exec(ctx, updateSQL, metadataJSON, time.Now(), accountID)
	if err != nil {
		r.logger.Error("failed to update account metadata",
			zap.Error(err),
			zap.String("account_id", accountID.String()))
		return fmt.Errorf("failed to update account metadata: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("account not found: %s", accountID)
	}

	r.logger.Info("account metadata updated",
		zap.String("account_id", accountID.String()))

	return nil
}
