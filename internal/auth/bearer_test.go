package auth

import (
	"encoding/base64"
	"path/filepath"
	"testing"
)

// Re-minting is the §7 revocation mechanism, so the file verifier must accept
// only the newest minted bearer without any process restart.
func TestRemintingRevokesThePreviousBearerWithoutRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bearer")
	first, err := Mint(MintOptions{Path: path})
	if err != nil {
		t.Fatalf("mint first bearer: %v", err)
	}
	verifier := FileVerifier{Path: path}
	if valid, verifyErr := verifier.Verify("Bearer " + first); verifyErr != nil || !valid {
		t.Fatalf("first minted bearer rejected: (%t,%v)", valid, verifyErr)
	}
	second, err := Mint(MintOptions{Path: path})
	if err != nil {
		t.Fatalf("re-mint bearer: %v", err)
	}
	if second == first {
		t.Fatal("re-mint returned the previous bearer")
	}
	if valid, verifyErr := verifier.Verify("Bearer " + first); verifyErr != nil || valid {
		t.Fatalf("revoked bearer still accepted: (%t,%v)", valid, verifyErr)
	}
	if valid, verifyErr := verifier.Verify("Bearer " + second); verifyErr != nil || !valid {
		t.Fatalf("re-minted bearer rejected: (%t,%v)", valid, verifyErr)
	}
}

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
