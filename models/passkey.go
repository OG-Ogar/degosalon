package models

import "time"

// Passkey is one WebAuthn credential registered against an account. The
// credential payload is the JSON form of the library record; it holds the
// public key and signature counter, never a private key or password.
type Passkey struct {
	CredentialID []byte
	Role         string
	UserID       int
	Email        string
	DisplayName  string
	Credential   []byte
	CreatedAt    time.Time
	LastUsedAt   time.Time
}

// RememberedDevice identifies an account that has signed in on a browser
// before. It is only a convenience hint for the login form: knowing the
// account email is never enough to sign in.
type RememberedDevice struct {
	TokenDigest string
	Role        string
	UserID      int
	ExpiresAt   time.Time
}
