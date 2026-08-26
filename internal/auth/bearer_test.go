package auth

import (
	"encoding/base64"
	"testing"
)

func TestAuthorizationAcceptsOnlyTheCanonicalMintedBearer(t *testing.T) {
	stored := [bearerBytes]byte{}
	encoded := base64.RawURLEncoding.EncodeToString(stored[:])

	if !verifyAuthorization("Bearer "+encoded, stored) {
		t.Fatal("canonical minted bearer was rejected")
	}

	alias := encoded[:len(encoded)-1] + "B"
	decodedAlias, err := base64.RawURLEncoding.DecodeString(alias)
	if err != nil || string(decodedAlias) != string(stored[:]) {
		t.Fatal("test fixture is not a non-canonical encoding of the stored bearer")
	}
	if verifyAuthorization("Bearer "+alias, stored) {
		t.Fatal("non-canonical encoding of the stored bearer was accepted")
	}
	if verifyAuthorization(encoded, stored) {
		t.Fatal("bearer without the exact authorization scheme was accepted")
	}
}
