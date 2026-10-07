package security

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// Independently checked with Python hashlib.pbkdf2_hmac('sha256',...).
func TestPBKDF2SHA256ReferenceVectors(t *testing.T) {
	for _, test := range []struct {
		iterations, length int
		want               string
	}{
		{1, 32, "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"},
		{2, 32, "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"},
		{2, 64, "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43830651afcb5c862f0b249bd031f7a67520d136470f5ec271ece91c07773253d9"},
	} {
		got := hex.EncodeToString(pbkdf2SHA256([]byte("password"), []byte("salt"), test.iterations, test.length))
		if got != test.want {
			t.Fatalf("iterations=%d length=%d: %s", test.iterations, test.length, got)
		}
	}
}

func TestPasswordHashesHaveFreshSaltAndVerifyExactInput(t *testing.T) {
	password := "Pāssword 🔑 with spaces"
	first, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("reused salt")
	}
	for _, digest := range []string{first, second} {
		if !VerifyPassword(digest, password) {
			t.Fatal("valid password rejected")
		}
		if VerifyPassword(digest, password+" ") || VerifyPassword(digest, "wrong-password") {
			t.Fatal("incorrect password accepted")
		}
		parts := strings.Split(digest, "$")
		if len(parts) != 4 || len(parts[2]) != saltBytes*2 || len(parts[3]) != keyBytes*2 {
			t.Fatal("unexpected digest format")
		}
	}
}

func TestMalformedPasswordDigestsAreRejectedBeforeHashing(t *testing.T) {
	salt := strings.Repeat("ab", saltBytes)
	key := strings.Repeat("cd", keyBytes)
	for _, digest := range []string{
		"", "unknown$1$" + salt + "$" + key, "pbkdf2_sha256$1$" + salt + "$" + key + "$extra",
		"pbkdf2_sha256$0$" + salt + "$" + key, "pbkdf2_sha256$-1$" + salt + "$" + key,
		"pbkdf2_sha256$invalid$" + salt + "$" + key,
		fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", maxVerifyIterations+1, salt, key),
		"pbkdf2_sha256$999999999999999999999999999$" + salt + "$" + key,
		"pbkdf2_sha256$1$$" + key, "pbkdf2_sha256$1$zz$" + key, "pbkdf2_sha256$1$ab$" + key,
		"pbkdf2_sha256$1$" + salt + "$", "pbkdf2_sha256$1$" + salt + "$zz", "pbkdf2_sha256$1$" + salt + "$cd",
		strings.Repeat("x", 257),
	} {
		if VerifyPassword(digest, "password") {
			t.Fatalf("accepted malformed digest %q", digest)
		}
	}
}

func FuzzVerifyMalformedPasswordDigest(f *testing.F) {
	for _, seed := range []string{"", "pbkdf2_sha256$0$$", "pbkdf2_sha256$2147483647$ab$cd"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, digest string) {
		// The fixed password's valid output is not supplied by the caller.
		_ = VerifyPassword(digest, "fuzz-password")
	})
}
