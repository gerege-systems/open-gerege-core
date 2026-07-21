// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"net/http"

	V1Handler "template/internal/http/handlers/v1"
)

// Нийтлэг body-хэмжээний дээд хязгаарууд. Route-ууд хүлээж авдаг
// payload-доо тохирох хамгийн чанга хязгаарыг хэрэглэдэг. Глобал
// өгөгдмөл (DefaultBodyMaxBytes) нь өөрийн хязгаар тогтоогоогүй аль ч
// route-ийн сүүлчийн хамгаалалтын шугам юм.
const (
	// DefaultBodyMaxBytes нь ердийн JSON API route-уудын дээд хязгаар — 1 MiB.
	// (DecodeBody мөн үүнтэй ижил 1 MiB-ийн гүний cap тавьдаг тул body уншилт
	// route-ийн хязгаараас үл хамааран 1 MiB-ээр хязгаарлагдана.)
	DefaultBodyMaxBytes int64 = 1 << 20

	// UploadBodyMaxBytes нь глобал root-ийн туйлын дээд хязгаар (hard ceiling).
	// Файл байршуулдаг цорын ганц route (/api/v1/sign/init multipart PDF ≤25 MB,
	// + /rp/sign relay) энэ хэмжээг шаарддаг тул глобал net-ийг үүгээр тавина;
	// ердийн JSON route-уудыг DecodeBody (1 MiB) + auth-ийн route-түвшний 4 KiB
	// cap нарийн хамгаална. Chi-д эцгийн middleware нь дэд route-ийн хязгаарыг
	// зөвхөн ЧАНГАЛЖ (tighten) чадна — СУЛРУУЛЖ чаддаггүй тул глобал 1 MiB нь
	// sign-ийн upload-ийг эцэгтээ 413-аар таслаж байсныг энэ засна.
	UploadBodyMaxBytes int64 = 26 << 20 // 25 MB + overhead (sign PDF)

	// AuthBodyMaxBytes нь register / login / refresh / logout payload-уудыг
	// хамардаг. Эдгээрийн аль нь ч хэдэн зуун байтаас илүү JSON авч
	// явдаггүй; 4 KiB-д хязгаарлах нь нэрээ нууцалсан урсгал хүлээн авдаг
	// цорын ганц route-уудын эсрэг хэт том payload-ийн дайралтыг хууль
	// ёсны ямар ч хүсэлтэд нөлөөлөхгүйгээр хаадаг.
	AuthBodyMaxBytes int64 = 4 << 10
)

// BodySizeLimitMiddleware нь body нь maxBytes-ээс хэтэрсэн аль ч
// хүсэлтийг 413 Payload Too Large-ээр татгалздаг. net/http-д бид
// r.Body-г http.MaxBytesReader-ээр ороодог тул хязгаараас хэтэрсэн body-г
// уншихыг оролдсон аль ч handler алдаа авдаг бөгөөд хэт том payload-г
// бүхэлд нь санах ойд буулгахаас сэргийлдэг. Server-түвшинд тогтоосон
// глобал хязгаар нь үнэхээр асар том upload-ийн эсрэг жинхэнэ эхний
// хамгаалалтын шугам; энэ нь түүн дээр route бүрийн чангалалт өгдөг.
func BodySizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Content-Length мэдэгдсэн бөгөөд хязгаараас хэтэрсэн бол body
			// уншихаас ӨМНӨ middleware түвшинд нэгдсэн 413 буцаана. (Урт нь
			// мэдэгдээгүй/chunked үед MaxBytesReader handler-ийн уншилт дээр
			// хязгаарлана.)
			if r.ContentLength > maxBytes {
				_ = V1Handler.NewErrorResponse(w, r, http.StatusRequestEntityTooLarge, "request entity too large")
				return
			}
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
