package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	bearerBytes       = 32
	bearerEncodedSize = 43
)

var bearerEncoding = base64.RawURLEncoding.Strict()

type MintOptions struct {
	Path string
}

type FileVerifier struct {
	Path string
}

func Mint(options MintOptions) (string, error) {
	if options.Path == "" {
		return "", errors.New("bearer file path is empty")
	}
	parent := filepath.Dir(options.Path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", fmt.Errorf("create bearer directory: %w", err)
	}

	raw := make([]byte, bearerBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("mint bearer: %w", err)
	}
	// justify-base64url-over-base64: this human-transferred mobile credential
	// avoids '+' and '/' transformations during phone credential entry.
	encoded := bearerEncoding.EncodeToString(raw)

	file, err := os.CreateTemp(parent, ".bearer-*")
	if err != nil {
		return "", fmt.Errorf("create bearer file: %w", err)
	}
	temporaryPath := file.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath) // justify-ignore-error: the original mint error is authoritative.
		}
	}()

	if err := file.Chmod(0o600); err != nil {
		_ = file.Close() // justify-ignore-error: chmod failure is authoritative.
		return "", fmt.Errorf("secure bearer file: %w", err)
	}
	if _, err := file.WriteString(encoded + "\n"); err != nil {
		_ = file.Close() // justify-ignore-error: write failure is authoritative.
		return "", fmt.Errorf("write bearer file: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close() // justify-ignore-error: sync failure is authoritative.
		return "", fmt.Errorf("sync bearer file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close bearer file: %w", err)
	}
	if err := os.Rename(temporaryPath, options.Path); err != nil {
		return "", fmt.Errorf("publish bearer file: %w", err)
	}
	removeTemporary = false
	return encoded, nil
}

func (verifier FileVerifier) Verify(authorization string) (bool, error) {
	stored, err := readBearer(verifier.Path)
	if err != nil {
		return false, err
	}
	return verifyAuthorization(authorization, stored), nil
}

func verifyAuthorization(authorization string, stored [bearerBytes]byte) bool {
	presented := [bearerBytes]byte{}
	valid := 1
	encoded, found := strings.CutPrefix(authorization, "Bearer ")
	if !found || len(encoded) != bearerEncodedSize {
		valid = 0
	} else {
		decoded, decodeErr := bearerEncoding.DecodeString(encoded)
		if decodeErr != nil || len(decoded) != bearerBytes {
			valid = 0
		} else {
			copy(presented[:], decoded)
		}
	}
	return subtle.ConstantTimeCompare(stored[:], presented[:])&valid == 1
}

func readBearer(path string) ([bearerBytes]byte, error) {
	var bearer [bearerBytes]byte
	if path == "" {
		return bearer, errors.New("bearer file path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return bearer, fmt.Errorf("stat bearer file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return bearer, errors.New("bearer file must be a regular file with mode 0600")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return bearer, fmt.Errorf("read bearer file: %w", err)
	}
	if len(contents) != bearerEncodedSize+1 || contents[len(contents)-1] != '\n' {
		return bearer, errors.New("bearer file has invalid contents")
	}
	decoded, err := bearerEncoding.DecodeString(string(contents[:len(contents)-1]))
	if err != nil || len(decoded) != bearerBytes {
		return bearer, errors.New("bearer file has invalid contents")
	}
	copy(bearer[:], decoded)
	return bearer, nil
}
