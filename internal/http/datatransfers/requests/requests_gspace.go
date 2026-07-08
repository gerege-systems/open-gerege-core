// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// GSpaceUploadRequest нь Gerege Space-д файл оруулах body. Data нь base64 (≤~2MB).
type GSpaceUploadRequest struct {
	Name string `json:"name" validate:"required,max=200"`
	Data string `json:"data" validate:"required,base64,max=2900000"`
}
