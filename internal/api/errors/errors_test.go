package errors

import (
	stderrors "errors"
	"testing"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service/transaction"
	"github.com/shopspring/decimal"
)

func TestConvertToAPIError_ValidationError(t *testing.T) {
	err := domain.ValidationError{Field: "state", Message: "invalid transaction state"}
	apiErr := ConvertToAPIError(err)

	if apiErr.Code != ErrorCodeInvalidState {
		t.Fatalf("expected %s, got %s", ErrorCodeInvalidState, apiErr.Code)
	}
}

func TestConvertToAPIError_InsufficientBalanceError(t *testing.T) {
	err := &transaction.InsufficientBalanceError{
		AccountID:        "acc-1",
		Currency:         "USD",
		RequiredAmount:   decimal.NewFromInt(100),
		AvailableBalance: decimal.NewFromInt(20),
	}

	apiErr := ConvertToAPIError(err)
	if apiErr.Code != ErrorCodeInsufficientBalance {
		t.Fatalf("expected %s, got %s", ErrorCodeInsufficientBalance, apiErr.Code)
	}
	if apiErr.Details["account_id"] != "acc-1" {
		t.Fatalf("expected account_id detail, got %v", apiErr.Details["account_id"])
	}
}

func TestConvertToAPIError_MessagePattern(t *testing.T) {
	err := stderrors.New("transaction not found: 123")
	apiErr := ConvertToAPIError(err)

	if apiErr.Code != ErrorCodeTransactionNotFound {
		t.Fatalf("expected %s, got %s", ErrorCodeTransactionNotFound, apiErr.Code)
	}
}

func TestConvertToAPIError_Nil(t *testing.T) {
	apiErr := ConvertToAPIError(nil)
	if apiErr.Code != ErrorCodeInternalError {
		t.Fatalf("expected %s, got %s", ErrorCodeInternalError, apiErr.Code)
	}
}
