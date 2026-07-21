// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package helpers

import "crypto/rand"

const otpPayloads = "0123456789"

func GenerateOTPCode(length int) (string, error) {
	otpCharsLength := byte(len(otpPayloads))
	// maxValid нь нэг байтад багтах otpCharsLength-ийн хамгийн том үржвэр бөгөөд
	// санамсаргүй байтуудыг OTP цифр рүү буулгахад modulo хазайлтыг арилгахад ашиглагдана.
	maxValid := 256 - (256 % int(otpCharsLength)) // 10 цифрийн хувьд 250

	result := make([]byte, length)
	buf := make([]byte, length+10) // rejection sampling-д зориулсан нэмэлт байтууд

	filled := 0
	for filled < length {
		_, err := rand.Read(buf)
		if err != nil {
			return "", err
		}
		for _, b := range buf {
			if filled >= length {
				break
			}
			if int(b) < maxValid {
				result[filled] = otpPayloads[b%otpCharsLength]
				filled++
			}
		}
	}

	return string(result), nil
}
