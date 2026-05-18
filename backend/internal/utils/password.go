// Package utils berisi helper-helper kecil yang reusable.
package utils

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"math/big"
)

const (
	upperChars   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerChars   = "abcdefghijklmnopqrstuvwxyz"
	digitChars   = "0123456789"
	specialChars = "!@#$%^&*-_=+"
)

// GenerateTemporaryPassword menghasilkan password acak yang
// kriptografis-aman, dijamin mengandung minimal 1 huruf besar,
// 1 huruf kecil, 1 angka, dan 1 karakter spesial.
func GenerateTemporaryPassword(length int) (string, error) {
	if length < 12 {
		return "", errors.New("panjang password minimal 12 karakter")
	}

	all := upperChars + lowerChars + digitChars + specialChars

	// Karakter wajib (1 dari setiap kategori)
	required := []byte{
		mustPick(upperChars),
		mustPick(lowerChars),
		mustPick(digitChars),
		mustPick(specialChars),
	}

	// Sisanya random dari semua kategori
	out := make([]byte, length)
	copy(out, required)
	for i := len(required); i < length; i++ {
		out[i] = mustPick(all)
	}

	// Acak posisi (Fisher-Yates) supaya karakter wajib tidak selalu di awal
	for i := length - 1; i > 0; i-- {
		j := mustIntn(i + 1)
		out[i], out[j] = out[j], out[i]
	}

	return string(out), nil
}

// ConstantTimeEqual membandingkan 2 string dalam waktu konstan
// untuk mencegah timing attack (mis. saat membandingkan token).
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// MaskEmail mengubah "john.doe@example.com" -> "jo***@example.com".
// Dipakai di response supaya email user lain tidak bocor.
func MaskEmail(email string) string {
	at := -1
	for i, c := range email {
		if c == '@' {
			at = i
			break
		}
	}
	if at <= 0 {
		return "***"
	}
	local := email[:at]
	domain := email[at:]
	if len(local) <= 2 {
		return string(local[0]) + "***" + domain
	}
	return local[:2] + "***" + domain
}

func mustPick(charset string) byte {
	idx := mustIntn(len(charset))
	return charset[idx]
}

func mustIntn(n int) int {
	if n <= 0 {
		return 0
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		// Sangat jarang terjadi; kalau crypto/rand gagal, sistem bermasalah.
		panic("crypto/rand failed: " + err.Error())
	}
	return int(v.Int64())
}
