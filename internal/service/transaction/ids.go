package transaction

import (
	"fmt"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/google/uuid"
)

func parseUUIDField(field, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, domain.ValidationError{
			Field:   field,
			Message: fmt.Sprintf("invalid %s format: %s", field, value),
		}
	}

	return id, nil
}
