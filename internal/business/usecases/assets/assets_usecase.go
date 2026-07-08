// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package assets нь хэрэглэгчийн гарын үсэг (хувь хүн) ба байгууллагын тамганы
// дардасын (байгууллага) зургийн URL-ийг удирдана. Зураг нь Google Drive-д
// (BFF талд хэрэглэгчийн холбосон токеноор) байршиж, энд зөвхөн URL хадгалагдана.
package assets

import "context"

type Usecase interface {
	// GetSignature нь нэвтэрсэн хэрэглэгчийн гарын үсгийн зургийн URL (эсвэл "").
	GetSignature(ctx context.Context, userID string) (string, error)
	// SetSignature нь гарын үсгийн зургийн URL-ийг тавина/шинэчилнэ.
	SetSignature(ctx context.Context, userID, url string) error
	// DeleteSignature нь гарын үсгийг устгана.
	DeleteSignature(ctx context.Context, userID string) error
	// GetStamp нь байгууллагын тамганы зургийн URL (эсвэл "").
	GetStamp(ctx context.Context, userID, orgRegister string) (string, error)
	// SetStamp нь байгууллагын тамгыг тавина — зөвхөн тухайн байгууллагын ADMIN.
	SetStamp(ctx context.Context, userID, orgRegister, url string) error
	// DeleteStamp нь байгууллагын тамгыг устгана — зөвхөн ADMIN.
	DeleteStamp(ctx context.Context, userID, orgRegister string) error
	// SetLatinName нь нэвтэрсэн хэрэглэгчийн латин нэрийг (first_name_en/last_name_en) засна.
	SetLatinName(ctx context.Context, userID, firstEn, lastEn string) error
	// SetOrgNameLatin нь байгууллагын латин нэрийг засна (eidmongolia талд ADMIN шалгана).
	SetOrgNameLatin(ctx context.Context, userID, orgRegister, nameLatin string) error
}
