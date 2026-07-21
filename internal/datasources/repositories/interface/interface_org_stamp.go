// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package _interface

import "context"

// OrgStampRepository нь байгууллагын тамганы дардасын зургийн URL-ийг (Google Drive)
// улсын бүртгэлийн дугаараар (org_register) хадгалах gateway. Эрхийн шалгалт
// (зөвхөн ADMIN тавьж/устгах) нь usecase давхаргад eID-ээр хийгдэнэ.
type OrgStampRepository interface {
	// Get нь тамганы URL-ийг буцаана; тавиагүй бол "".
	Get(ctx context.Context, orgRegister string) (string, error)
	// Upsert нь тамганы URL-ийг тавина/шинэчилнэ (uploadedBy — тавьсан хэрэглэгчийн ID).
	Upsert(ctx context.Context, orgRegister, url, uploadedBy string) error
	// Delete нь тамгыг устгана.
	Delete(ctx context.Context, orgRegister string) error
}
