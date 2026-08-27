package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	bearerBytes        = 32
	bearerEncodedSize  = 43
	bearerDigestDomain = "skidbladnir.gateway-bearer.v1\x00"
)

var bearerEncoding = base64.RawURLEncoding.Strict()

type MintOptions struct {
	Path string
}

type FileVerifier struct {
	Path string
}

type Credential struct {
	raw [bearerBytes]byte
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
	credential, err := verifier.Read()
	if err != nil {
		return false, err
	}
	return credential.Verify(authorization), nil
}

func (verifier FileVerifier) Read() (Credential, error) {
	return readBearer(verifier.Path)
}

func (credential Credential) CanonicalBearer() string {
	return bearerEncoding.EncodeToString(credential.raw[:])
}

func (credential Credential) Digest() [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte(bearerDigestDomain)) // justify-ignore-error: hash.Hash.Write never returns an error.
	_, _ = hash.Write(credential.raw[:])          // justify-ignore-error: hash.Hash.Write never returns an error.
	var verifier [sha256.Size]byte
	copy(verifier[:], hash.Sum(nil))
	return verifier
}

func (credential Credential) Verify(authorization string) bool {
	return verifyAuthorization(authorization, credential.raw)
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

func readBearer(path string) (Credential, error) {
	if path == "" {
		return Credential{}, errors.New("bearer file path is empty")
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return Credential{}, fmt.Errorf("inspect bearer file: %w", err)
	}
	if !secureBearerFile(pathInfo.Mode()) {
		return Credential{}, errors.New("bearer file must be a regular file with mode 0600")
	}
	file, err := os.Open(path)
	if err != nil {
		return Credential{}, fmt.Errorf("open bearer file: %w", err)
	}
	return readOpenedBearer(pathInfo, file)
}

func readOpenedBearer(pathInfo fs.FileInfo, file *os.File) (Credential, error) {
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close() // justify-ignore-error: the stat failure is authoritative.
		return Credential{}, fmt.Errorf("inspect open bearer file: %w", err)
	}
	if !secureBearerFile(openedInfo.Mode()) || !os.SameFile(pathInfo, openedInfo) {
		_ = file.Close() // justify-ignore-error: the changed or insecure file is authoritative.
		return Credential{}, errors.New("bearer file changed while opening")
	}
	contents, err := io.ReadAll(io.LimitReader(file, bearerEncodedSize+2))
	if err != nil {
		_ = file.Close() // justify-ignore-error: the read failure is authoritative.
		return Credential{}, fmt.Errorf("read bearer file: %w", err)
	}
	if err := file.Close(); err != nil {
		return Credential{}, fmt.Errorf("close bearer file: %w", err)
	}
	if len(contents) != bearerEncodedSize+1 || contents[len(contents)-1] != '\n' {
		return Credential{}, errors.New("bearer file has invalid contents")
	}
	decoded, err := bearerEncoding.DecodeString(string(contents[:len(contents)-1]))
	if err != nil || len(decoded) != bearerBytes {
		return Credential{}, errors.New("bearer file has invalid contents")
	}
	var credential Credential
	copy(credential.raw[:], decoded)
	return credential, nil
}

func secureBearerFile(mode os.FileMode) bool {
	return mode.IsRegular() && mode.Perm() == 0o600
}
