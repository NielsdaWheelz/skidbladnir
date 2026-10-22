package install

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

//go:embed release-public-key.pem
var publicPEM []byte
var versionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func verify(archive, signature string) error {
	block, _ := pem.Decode(publicPEM)
	if block == nil {
		return errors.New("release signing key is invalid")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	pub, ok := key.(ed25519.PublicKey)
	if !ok {
		return errors.New("release signing key is not ed25519")
	}
	// signatures cover the sha256 digest, permitting bounded-memory verification of large archives.
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(signature)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, hash.Sum(nil), sig) {
		return errors.New("release signature invalid; installed release unchanged")
	}
	return nil
}

func extract(archive, destination string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	zipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer zipped.Close()
	reader := tar.NewReader(zipped)
	// The signature proves the layout. These checks keep the installer's writes and
	// the release's relative links inside destination, which check-release does not
	// fully prove. Exclusive creation rejects a duplicated file or link.
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(header.Name, "/")
		clean := filepath.Clean(filepath.FromSlash(name))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("release archive contains an invalid path")
		}
		path := filepath.Join(destination, clean)
		// Never follow an archive-created symlink while extracting later members.
		parent := filepath.Dir(path)
		for parent != destination {
			info, e := os.Lstat(parent)
			if e == nil && info.Mode()&os.ModeSymlink != 0 {
				return errors.New("release archive traverses a symlink")
			}
			if e != nil && !errors.Is(e, os.ErrNotExist) {
				return e
			}
			parent = filepath.Dir(parent)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
			if err := os.Chmod(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode)&0755)
			if e != nil {
				return e
			}
			_, e = io.Copy(f, reader)
			if e == nil {
				e = f.Chmod(os.FileMode(header.Mode) & 0755)
			}
			if e = errors.Join(e, f.Close()); e != nil {
				return e
			}
		case tar.TypeSymlink:
			target := filepath.Clean(filepath.Join(filepath.Dir(clean), header.Linkname))
			if filepath.IsAbs(header.Linkname) || target == ".." || strings.HasPrefix(target, "../") {
				return errors.New("release symlink escapes archive")
			}
			if e := os.Symlink(header.Linkname, path); e != nil {
				return e
			}
		default:
			return errors.New("release archive contains unsupported member type")
		}
	}
}

func download(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("release download failed: http %d", res.StatusCode)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, io.LimitReader(res.Body, 2*1024*1024*1024))
	return errors.Join(err, f.Close())
}

func latest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/NielsdaWheelz/skidbladnir/releases/latest", nil)
	if err != nil {
		return "", err
	}
	res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("release lookup failed: http %d", res.StatusCode)
	}
	var body struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1024*1024)).Decode(&body); err != nil {
		return "", err
	}
	if !versionPattern.MatchString(body.Tag) {
		return "", errors.New("latest release has invalid version")
	}
	return body.Tag, nil
}
