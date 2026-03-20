package transaction

import (
	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/repository"
)

func (p *PostgresTransactionProcessor) requestToTransaction(req PostTransactionRequest) *domain.Transaction {
	entries := make([]domain.Entry, len(req.Entries))
	for i, e := range req.Entries {
		entries[i] = domain.Entry{
			AccountID:    e.AccountID,
			CurrencyCode: e.CurrencyCode,
			Amount:       e.Amount,
			EntryType:    e.EntryType,
			Description:  e.Description,
		}
	}

	return &domain.Transaction{
		State:    req.State,
		Entries:  entries,
		Metadata: req.Metadata,
	}
}

func entryRequestsToLedgerInputs(reqs []EntryRequest) []repository.LedgerEntryInput {
	out := make([]repository.LedgerEntryInput, len(reqs))
	for i, e := range reqs {
		out[i] = repository.LedgerEntryInput{
			AccountID:    e.AccountID,
			CurrencyCode: e.CurrencyCode,
			Amount:       e.Amount,
			EntryType:    e.EntryType,
			Description:  e.Description,
		}
	}
	return out
}
