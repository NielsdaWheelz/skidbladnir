package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"strings"
)

// NewCredential mints one device credential without publishing it to a file.
func NewCredential() (Credential, error) { var c Credential; _, e := rand.Read(c.raw[:]); return c, e }
func ParseCredential(value string) (Credential, error) {
	var c Credential
	b, e := bearerEncoding.DecodeString(value)
	if e != nil || len(b) != bearerBytes {
		return c, errors.New("credential is invalid")
	}
	copy(c.raw[:], b)
	return c, nil
}

// Verifier is sha256 of the random bearer bytes, shared without the credential.
func (c Credential) Verifier() string {
	sum := sha256.Sum256(c.raw[:])
	return bearerEncoding.EncodeToString(sum[:])
}
func ValidVerifier(value string) bool {
	b, e := bearerEncoding.DecodeString(value)
	return e == nil && len(b) == sha256.Size
}
func VerifyDigest(verifier, authorization string) bool {
	raw, ok := strings.CutPrefix(authorization, "Bearer ")
	c, e := ParseCredential(raw)
	valid := 0
	if ok && e == nil {
		valid = 1
	}
	a := c.Verifier()
	return subtle.ConstantTimeCompare([]byte(a), []byte(verifier))&valid == 1
}
