package auth

import (
	"encoding/base64"
	"os"
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

func TestBearerReaderRejectsASymlinkEvenWhenItsTargetIsSecure(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "bearer")
	if _, err := Mint(MintOptions{Path: target}); err != nil {
		t.Fatalf("mint target bearer: %v", err)
	}
	link := filepath.Join(directory, "bearer-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create bearer symlink: %v", err)
	}
	if _, err := (FileVerifier{Path: link}).Read(); err == nil {
		t.Fatal("bearer reader accepted a symlink to a mode-0600 target")
	}
}

func TestBearerReaderRejectsAnOpenedDescriptorFromAnotherPath(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "first")
	second := filepath.Join(directory, "second")
	if _, err := Mint(MintOptions{Path: first}); err != nil {
		t.Fatalf("mint first bearer: %v", err)
	}
	if _, err := Mint(MintOptions{Path: second}); err != nil {
		t.Fatalf("mint second bearer: %v", err)
	}
	pathInfo, err := os.Lstat(first)
	if err != nil {
		t.Fatalf("inspect first bearer: %v", err)
	}
	opened, err := os.Open(second)
	if err != nil {
		t.Fatalf("open second bearer: %v", err)
	}
	if _, err := readOpenedBearer(pathInfo, opened); err == nil {
		t.Fatal("bearer reader accepted a descriptor that did not match the inspected path")
	}
}

func TestBearerReaderRevalidatesModeOnTheOpenedDescriptor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bearer")
	if _, err := Mint(MintOptions{Path: path}); err != nil {
		t.Fatalf("mint bearer: %v", err)
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("inspect bearer: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("make bearer insecure after inspection: %v", err)
	}
	opened, err := os.Open(path)
	if err != nil {
		t.Fatalf("open bearer: %v", err)
	}
	if _, err := readOpenedBearer(pathInfo, opened); err == nil {
		t.Fatal("bearer reader accepted an opened descriptor whose mode changed after inspection")
	}
}
