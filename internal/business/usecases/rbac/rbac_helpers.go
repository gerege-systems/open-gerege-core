// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package rbac

import (
	"errors"
	"fmt"
	"strings"

	"template/internal/apperror"
)

func mapRepoError(err error, op string) error {
	if err == nil {
		return nil
	}
	var de *apperror.DomainError
	if errors.As(err, &de) {
		return err
	}
	return apperror.InternalCause(fmt.Errorf("%s: %w", op, err))
}

// slugifyKey нь эрхийн түлхүүрийг үүсгэнэ — key хоосон бол name-ээс гарган,
// жижиг үсэг + alnum/_ болгоно (жишээ нь "Sales Manager" → "sales_manager").
func slugifyKey(key, name string) string {
	s := strings.TrimSpace(key)
	if s == "" {
		s = name
	}
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevUnderscore = false
		case r == '_' || r == '-' || r == ' ':
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}
