package query

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestNewPostgresQueryService(t *testing.T) {
	svc := NewPostgresQueryService(nil, nil, zap.NewNop())
	if svc == nil {
		t.Fatal("expected non-nil query service")
	}
}

func TestGetTransaction_InvalidID(t *testing.T) {
	svc := NewPostgresQueryService(nil, nil, zap.NewNop())
	_, err := svc.GetTransaction(context.Background(), "bad-uuid")
	if err == nil {
		t.Fatal("expected invalid id error")
	}
}
