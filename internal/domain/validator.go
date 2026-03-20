package domain

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

type TransactionValidator struct {
	supportedCurrencies  map[string]bool
	maxDecimalPlaces     int32
	maxMetadataKeySize   int
	maxMetadataValueSize int
}

func NewTransactionValidator(supportedCurrencies []string) *TransactionValidator {
	currencyMap := make(map[string]bool)
	for _, currency := range supportedCurrencies {
		currencyMap[currency] = true
	}

	return &TransactionValidator{
		supportedCurrencies:  currencyMap,
		maxDecimalPlaces:     8,
		maxMetadataKeySize:   100,
		maxMetadataValueSize: 1000,
	}
}

func (v *TransactionValidator) ValidateTransaction(txn *Transaction) error {
	if err := v.validateRequiredFields(txn); err != nil {
		return err
	}

	// Validate entries
	if err := v.validateEntries(txn.Entries); err != nil {
		return err
	}

	// Validate double-entry balance
	if err := v.validateDoubleEntryBalance(txn.Entries); err != nil {
		return err
	}

	// Validate metadata
	if err := v.validateMetadata(txn.Metadata); err != nil {
		return err
	}

	return nil
}

func (v *TransactionValidator) validateRequiredFields(txn *Transaction) error {
	if txn.Entries == nil || len(txn.Entries) == 0 {
		return ValidationError{
			Field:   "entries",
			Message: "transaction must have at least one entry",
		}
	}

	if !txn.State.IsValid() {
		return ValidationError{
			Field:   "state",
			Message: fmt.Sprintf("invalid transaction state : %s", txn.State),
		}
	}

	return nil
}

func (v *TransactionValidator) validateEntries(entries []Entry) error {
	if len(entries) == 0 {
		return ValidationError{
			Field:   "entries",
			Message: "transaction must have at least one entry",
		}
	}

	for i, entry := range entries {
		if err := v.validateEntry(&entry, i); err != nil {
			return err
		}
	}

	return nil
}

func (v *TransactionValidator) validateEntry(entry *Entry, index int) error {
	fieldPrefix := fmt.Sprintf("entries[%d]", index)

	if entry.AccountID == "" {
		return ValidationError{
			Field:   fmt.Sprintf("%s.account_id", fieldPrefix),
			Message: "account_id is required",
		}
	}

	if entry.CurrencyCode == "" {
		return ValidationError{
			Field:   fmt.Sprintf("%s.currency_code", fieldPrefix),
			Message: "currency_code is required",
		}
	}

	if err := v.validateCurrency(entry.CurrencyCode); err != nil {
		return ValidationError{
			Field:   fmt.Sprintf("%s.currency_code", fieldPrefix),
			Message: err.Error(),
		}
	}

	if !entry.EntryType.IsValid() {
		return ValidationError{
			Field:   fmt.Sprintf("%s.entry_type", fieldPrefix),
			Message: fmt.Sprintf("invalid entry type: %s", entry.EntryType),
		}
	}

	if entry.Amount.IsNegative() {
		return ValidationError{
			Field:   fmt.Sprintf("%s.amount", fieldPrefix),
			Message: "amount must be non-negative",
		}
	}

	if err := v.validatePrecision(entry.Amount, fieldPrefix); err != nil {
		return err
	}

	return nil
}

func (v *TransactionValidator) validateCurrency(currencyCode string) error {
	// First check if it's a valid format (3 uppercase letters)
	currency := CurrencyCode(currencyCode)
	if !currency.IsValid() {
		return fmt.Errorf("invalid currency code format: %s (must be 3 uppercase letters)", currencyCode)
	}

	// Check if it's in the supported list
	if !v.supportedCurrencies[currencyCode] {
		return fmt.Errorf("unsupported currency code: %s", currencyCode)
	}

	return nil
}

func (v *TransactionValidator) validatePrecision(amount decimal.Decimal, fieldPrefix string) error {
	// Get the exponent (negative value indicates decimal places)
	exponent := amount.Exponent()

	// If exponent is positive or zero, no decimal places
	if exponent >= 0 {
		return nil
	}

	// Check if decimal places exceed maximum
	decimalPlaces := -exponent
	if decimalPlaces > v.maxDecimalPlaces {
		return ValidationError{
			Field:   fmt.Sprintf("%s.amount", fieldPrefix),
			Message: fmt.Sprintf("amount has %d decimal places, maximum allowed is %d", decimalPlaces, v.maxDecimalPlaces),
		}
	}

	return nil
}

func (v *TransactionValidator) validateDoubleEntryBalance(entries []Entry) error {
	totalDebits := decimal.Zero
	totalCredits := decimal.Zero

	for _, entry := range entries {
		switch entry.EntryType {
		case EntryTypeDebit:
			totalDebits = totalDebits.Add(entry.Amount)
		case EntryTypeCredit:
			totalCredits = totalCredits.Add(entry.Amount)
		}
	}

	if !totalDebits.Equal(totalCredits) {
		return ValidationError{
			Field: "entries",
			Message: fmt.Sprintf("transaction is not balanced: total debits (%s) must equal total credits (%s)",
				totalDebits.String(), totalCredits.String()),
		}
	}

	return nil
}

func (v *TransactionValidator) validateMetadata(metadata map[string]interface{}) error {
	if metadata == nil {
		return nil // Metadata is optional
	}

	for key, value := range metadata {
		// Validate key size
		if len(key) > v.maxMetadataKeySize {
			return ValidationError{
				Field:   "metadata",
				Message: fmt.Sprintf("metadata key '%s' exceeds maximum size of %d characters", key, v.maxMetadataKeySize),
			}
		}

		// Validate value size (convert to string for size check)
		valueStr := fmt.Sprintf("%v", value)
		if len(valueStr) > v.maxMetadataValueSize {
			return ValidationError{
				Field:   "metadata",
				Message: fmt.Sprintf("metadata value for key '%s' exceeds maximum size of %d characters", key, v.maxMetadataValueSize),
			}
		}
	}

	return nil
}
