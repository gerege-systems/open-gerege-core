// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package core нь Gerege Core (core.gerege.mn)-ийн USER FIND / ORG FIND
// үйлчилгээг wrap хийнэ. core.gerege.mn-ий хариуг (json) дамжуулна.
package core

import (
	"context"
	"encoding/json"
)

type Usecase interface {
	// FindUsers нь core.gerege.mn /api/user/find руу search_text-ээр хайна
	// (core_id эсвэл регистрийн дугаар). Хариуг raw JSON-оор буцаана.
	FindUsers(ctx context.Context, searchText string) (json.RawMessage, error)
	// FindOrganizations нь core.gerege.mn /api/organization/find руу хайна.
	FindOrganizations(ctx context.Context, searchText string) (json.RawMessage, error)
}
