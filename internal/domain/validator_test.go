package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestValidateTransaction_Success(t *testing.T) {
	v := NewTransactionValidator([]string{"USD"})
	txn := &Transaction{
		State: TransactionStateSettled,
		Entries: []Entry{
			{AccountID: "a1", CurrencyCode: "USD", Amount: decimal.NewFromInt(100), EntryType: EntryTypeDebit},
			{AccountID: "a2", CurrencyCode: "USD", Amount: decimal.NewFromInt(100), EntryType: EntryTypeCredit},
		},
	}

	if err := v.ValidateTransaction(txn); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateTransaction_Unbalanced(t *testing.T) {
	v := NewTransactionValidator([]string{"USD"})
	txn := &Transaction{
		State: TransactionStateSettled,
		Entries: []Entry{
			{AccountID: "a1", CurrencyCode: "USD", Amount: decimal.NewFromInt(100), EntryType: EntryTypeDebit},
			{AccountID: "a2", CurrencyCode: "USD", Amount: decimal.NewFromInt(90), EntryType: EntryTypeCredit},
		},
	}

	err := v.ValidateTransaction(txn)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Field != "entries" {
		t.Fatalf("expected field entries, got %s", validationErr.Field)
	}
}

func TestValidateTransaction_UnsupportedCurrency(t *testing.T) {
	v := NewTransactionValidator([]string{"USD"})
	txn := &Transaction{
		State: TransactionStateSettled,
		Entries: []Entry{
			{AccountID: "a1", CurrencyCode: "EUR", Amount: decimal.NewFromInt(100), EntryType: EntryTypeDebit},
			{AccountID: "a2", CurrencyCode: "EUR", Amount: decimal.NewFromInt(100), EntryType: EntryTypeCredit},
		},
	}

	err := v.ValidateTransaction(txn)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestValidateTransaction_MaxPrecision(t *testing.T) {
	v := NewTransactionValidator([]string{"USD"})
	txn := &Transaction{
		State: TransactionStateSettled,
		Entries: []Entry{
			{AccountID: "a1", CurrencyCode: "USD", Amount: decimal.RequireFromString("1.123456789"), EntryType: EntryTypeDebit},
			{AccountID: "a2", CurrencyCode: "USD", Amount: decimal.RequireFromString("1.123456789"), EntryType: EntryTypeCredit},
		},
	}

	err := v.ValidateTransaction(txn)
	if err == nil {
		t.Fatal("expected precision validation error, got nil")
	}
}
