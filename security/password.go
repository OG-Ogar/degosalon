// Package security holds the password hashing used by every login in
// the application.
//
// Passwords are stretched with PBKDF2-HMAC-SHA256 (RFC 8018) using a
// fresh random salt per password, and compared in constant time. This
// keeps the project dependency-free; bcrypt, scrypt or Argon2id from
// golang.org/x/crypto are stronger choices if you are willing to add
// that module.
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	algorithm           = "pbkdf2_sha256"
	iterations          = 150000
	saltBytes           = 16
	keyBytes            = 32
	maxVerifyIterations = 1000000
)

func getIterations() int {
	if os.Getenv("APP_ENV") == "test" {
		return 1000
	}
	return iterations
}

// referenceHash is hashed against in DummyVerify so that a login
// attempt for an unknown account costs roughly the same as one for a
// real account. It is the hash of a value nobody can log in with.
var referenceHash, _ = HashPassword("never-a-real-password")

// HashPassword returns an encoded digest of password in the format
// "pbkdf2_sha256$iterations$salt$key", all hex encoded. Every call
// returns a different string for the same input because the salt is
// random.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltBytes)

	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	iter := getIterations()
	key := pbkdf2SHA256([]byte(password), salt, iter, keyBytes)

	return fmt.Sprintf(
		"%s$%d$%s$%s",
		algorithm,
		iter,
		hex.EncodeToString(salt),
		hex.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the encoded digest.
// A malformed or empty digest never matches.
func VerifyPassword(encoded, password string) bool {
	// Bound work and allocations before decoding an untrusted/corrupt digest.
	if len(encoded) > 256 {
		return false
	}
	parts := strings.Split(encoded, "$")

	if len(parts) != 4 || parts[0] != algorithm {
		return false
	}

	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 || iter > maxVerifyIterations {
		return false
	}

	salt, err := hex.DecodeString(parts[2])
	if err != nil || len(salt) != saltBytes {
		return false
	}

	want, err := hex.DecodeString(parts[3])
	if err != nil || len(want) != keyBytes {
		return false
	}

	got := pbkdf2SHA256([]byte(password), salt, iter, len(want))

	return subtle.ConstantTimeCompare(got, want) == 1
}

// DummyVerify burns the same work as VerifyPassword without ever
// succeeding. Login handlers call it when the submitted email matches
// no account, so response time does not reveal which emails exist.
func DummyVerify(password string) {
	_ = VerifyPassword(referenceHash, password)
}

// pbkdf2SHA256 derives keyLen bytes from password and salt, per
// RFC 8018 section 5.2 with HMAC-SHA256 as the pseudorandom function.
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLen := prf.Size()
	blocks := (keyLen + hashLen - 1) / hashLen

	out := make([]byte, 0, blocks*hashLen)
	counter := make([]byte, 4)
	u := make([]byte, 0, hashLen)
	t := make([]byte, hashLen)

	for block := 1; block <= blocks; block++ {

		// U1 = PRF(password, salt || INT_BE32(block))
		prf.Reset()
		prf.Write(salt)
		binary.BigEndian.PutUint32(counter, uint32(block))
		prf.Write(counter)
		u = prf.Sum(u[:0])
		copy(t, u)

		// T = U1 xor U2 xor ... xor Uiter
		for n := 2; n <= iter; n++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])

			for i := range t {
				t[i] ^= u[i]
			}
		}

		out = append(out, t...)
	}

	return out[:keyLen]
}
