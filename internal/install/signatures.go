package install

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"debug/macho"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed mac-signing-cert.sha256
var macSigningCertificate string

func verifyMacSignature(ctx context.Context, path, identifier string) (string, error) {
	if err := exec.CommandContext(ctx, "codesign", "--verify", "--strict", path).Run(); err != nil {
		return "", fmt.Errorf("invalid mac artifact signature for %s: %w", identifier, err)
	}
	scratch, err := os.MkdirTemp("", "skid-signature-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	if err = exec.CommandContext(ctx, "codesign", "--display", "--extract-certificates="+filepath.Join(scratch, "cert"), path).Run(); err != nil {
		return "", err
	}
	certificate, err := os.ReadFile(filepath.Join(scratch, "cert0"))
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(certificate)
	if hex.EncodeToString(digest[:]) != strings.TrimSpace(macSigningCertificate) {
		return "", errors.New("mac artifact signer differs from the pinned certificate")
	}
	details, err := exec.CommandContext(ctx, "codesign", "--display", "--verbose=4", path).CombinedOutput()
	if err != nil {
		return "", err
	}
	found := false
	for _, line := range strings.Split(string(details), "\n") {
		if line == "Identifier="+identifier {
			found = true
		}
	}
	if !found {
		return "", errors.New("mac artifact signing identifier differs")
	}
	requirement, err := exec.CommandContext(ctx, "codesign", "--display", "-r-", path).Output()
	if err != nil {
		return "", err
	}
	leaf := sha1.Sum(certificate)
	expected := fmt.Sprintf("designated => identifier %q and certificate leaf = H%q", identifier, hex.EncodeToString(leaf[:]))
	if strings.TrimSpace(string(requirement)) != expected {
		return "", errors.New("mac artifact designated requirement differs")
	}
	return expected, nil
}

func sameMacRuntime(ctx context.Context, installed, candidate string) (bool, error) {
	oldRequirement, err := verifyMacSignature(ctx, installed, "dev.niels.skidbladnir.terminals")
	if err != nil {
		return false, err
	}
	newRequirement, err := verifyMacSignature(ctx, candidate, "dev.niels.skidbladnir.terminals")
	if err != nil {
		return false, err
	}
	if oldRequirement != newRequirement {
		return false, nil
	}
	scratch, err := os.MkdirTemp("", "skid-runtime-compare-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(scratch)
	resources := make([]map[string][32]byte, 2)
	for index, app := range []string{installed, candidate} {
		resources[index] = map[string][32]byte{}
		err = filepath.WalkDir(app, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(app, path)
			if err != nil {
				return err
			}
			if relative == "Contents/_CodeSignature" {
				return filepath.SkipDir
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if relative == "Contents/MacOS/skid-terminals" {
				private := filepath.Join(scratch, fmt.Sprintf("runtime-%d", index))
				if err = os.WriteFile(private, data, 0700); err != nil {
					return err
				}
				if err = exec.CommandContext(ctx, "codesign", "--force", "--sign", "-", "--timestamp=none", "--options", "0", "--identifier", "dev.niels.skidbladnir.reproduction", private).Run(); err != nil {
					return err
				}
				if err = exec.CommandContext(ctx, "codesign", "--remove-signature", private).Run(); err != nil {
					return err
				}
				data, err = os.ReadFile(private)
				if err != nil {
					return err
				}
				if err = normalizeRuntimeBuildMetadata(data); err != nil {
					return err
				}
			}
			resources[index][relative] = sha256.Sum256(data)
			return nil
		})
		if err != nil {
			return false, err
		}
	}
	return maps.Equal(resources[0], resources[1]), nil
}

// normalizeRuntimeBuildMetadata clears only Go build identity and its Mach-O UUID
// in the private byte buffer. It performs no I/O and preserves every other byte.
func normalizeRuntimeBuildMetadata(data []byte) error {
	image, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	text := image.Section("__text")
	if image.Magic != macho.Magic64 || text == nil {
		return errors.New("terminal runtime is not a Go Mach-O executable")
	}
	if uint64(text.Offset) > uint64(len(data)) || text.Size > uint64(len(data))-uint64(text.Offset) {
		return errors.New("terminal runtime has an invalid text section")
	}
	// Go puts its framed build ID at __text's start and derives LC_UUID
	// from it. Neither identifies executable behavior; compare all other
	// bytes, including the metadata's framing and length, unchanged.
	body := data[text.Offset : uint64(text.Offset)+text.Size]
	prefix := []byte("\xff Go build ID: \"")
	if !bytes.HasPrefix(body, prefix) {
		return errors.New("terminal runtime lacks its Go build ID header")
	}
	end := bytes.Index(body[len(prefix):], []byte("\"\n \xff"))
	if end <= 0 {
		return errors.New("terminal runtime has an incomplete Go build ID")
	}
	if _, err = strconv.Unquote(string(body[len(prefix)-1 : len(prefix)+end+1])); err != nil {
		return errors.New("terminal runtime has an invalid Go build ID")
	}
	clear(body[len(prefix) : len(prefix)+end])
	offset, uuid := 32, false // Mach-O 64-bit header size.
	for _, load := range image.Loads {
		raw := load.Raw()
		if image.ByteOrder.Uint32(raw) == 0x1b { // LC_UUID
			if uuid || len(raw) != 24 {
				return errors.New("terminal runtime has an invalid Mach-O UUID")
			}
			clear(data[offset+8 : offset+24])
			uuid = true
		}
		offset += len(raw)
	}
	if !uuid {
		return errors.New("terminal runtime lacks its Mach-O UUID")
	}
	return nil
}
