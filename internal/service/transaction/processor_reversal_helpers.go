package transaction

import (
	"fmt"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
)

func (p *PostgresTransactionProcessor) validateReversalEligibility(originalTxn *domain.Transaction) error {
	if originalTxn.ReversedByTransactionID != nil {
		return domain.ValidationError{
			Field:   "original_transaction_id",
			Message: fmt.Sprintf("transaction %s is already reversed by transaction %s", originalTxn.TransactionID, *originalTxn.ReversedByTransactionID),
		}
	}

	if originalTxn.ReversesTransactionID != nil {
		return domain.ValidationError{
			Field:   "original_transaction_id",
			Message: fmt.Sprintf("cannot reverse a reversal transaction %s", originalTxn.TransactionID),
		}
	}

	if originalTxn.State != domain.TransactionStateSettled {
		return domain.ValidationError{
			Field:   "original_transaction_id",
			Message: fmt.Sprintf("can only reverse settled transactions, but transaction %s is in state %s", originalTxn.TransactionID, originalTxn.State),
		}
	}

	return nil
}

func (p *PostgresTransactionProcessor) createReversalEntries(originalEntries []domain.Entry, description string) []EntryRequest {
	reversalEntries := make([]EntryRequest, len(originalEntries))

	for i, originalEntry := range originalEntries {
		var reversalEntryType domain.EntryType
		if originalEntry.EntryType == domain.EntryTypeDebit {
			reversalEntryType = domain.EntryTypeCredit
		} else {
			reversalEntryType = domain.EntryTypeDebit
		}

		reversalDescription := description
		if reversalDescription == "" {
			reversalDescription = fmt.Sprintf("Reversal of %s: %s", originalEntry.EntryType, originalEntry.Description)
		}

		reversalEntries[i] = EntryRequest{
			AccountID:    originalEntry.AccountID,
			CurrencyCode: originalEntry.CurrencyCode,
			Amount:       originalEntry.Amount,
			EntryType:    reversalEntryType,
			Description:  reversalDescription,
		}
	}

	return reversalEntries
}
