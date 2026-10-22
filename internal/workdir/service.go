package workdir

import (
	"cmp"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"unicode/utf8"
)

const (
	maximumPathBytes     = 4 * 1024
	maximumScanned       = 4 * 1024
	maximumChildren      = 256
	maximumPathTextBytes = 32 * 1024
	readBatchSize        = 64
)

type ErrorCode string

const (
	Invalid     ErrorCode = "Invalid"
	Unavailable ErrorCode = "Unavailable"
	TooLarge    ErrorCode = "TooLarge"
)

type operationError struct{ code ErrorCode }

func (failure *operationError) Error() string { return "working directory operation failed" }

func ErrorCodeOf(err error) (ErrorCode, bool) {
	var failure *operationError
	if !errors.As(err, &failure) {
		return "", false
	}
	return failure.code, true
}

// Listing is one browse token's child directories, in its wire shape.
type Listing struct {
	Directory       string  `json:"directory"`
	ParentDirectory string  `json:"parentDirectory,omitempty"` // "" only for "~"
	Children        []Entry `json:"children"`                  // never nil
	Omitted         bool    `json:"omitted"`
}

type Entry struct {
	Directory string `json:"directory"`
	Kind      string `json:"kind"` // "Directory" | "SymbolicLink"
}

type Service struct {
	home       string
	zoxidePath string
}

func New(home, zoxidePath string) (*Service, error) {
	if !validPathText(home) || !filepath.IsAbs(home) || filepath.Clean(home) != home {
		return nil, newError(Invalid)
	}
	if CheckStart(home) != nil {
		return nil, newError(Unavailable)
	}
	if zoxidePath != "" && (!validPathText(zoxidePath) || !filepath.IsAbs(zoxidePath) || filepath.Clean(zoxidePath) != zoxidePath) {
		return nil, newError(Invalid)
	}
	return &Service{home: home, zoxidePath: zoxidePath}, nil
}

// ParseStart canonicalizes a start directory: exact ~ or ~/x expand under Home,
// anything else must be absolute; filepath.Clean once. Fails Invalid.
func (service *Service) ParseStart(value string) (string, error) {
	if !validPathText(value) {
		return "", newError(Invalid)
	}
	expanded := value
	switch {
	case value == "~":
		expanded = service.home
	case strings.HasPrefix(value, "~/"):
		expanded = service.home + string(filepath.Separator) + value[2:]
	case !filepath.IsAbs(value):
		return "", newError(Invalid)
	}
	normalized := filepath.Clean(expanded)
	if !validPathText(normalized) {
		return "", newError(Invalid)
	}
	return normalized, nil
}

// CheckStart fails Unavailable unless path is a directory this user may enter.
// The error is content-free and wraps no errno: recovery classifies errno first.
func CheckStart(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() || syscall.Access(path, 1) != nil {
		return newError(Unavailable)
	}
	return nil
}

// List reads a browse token's child directories: exactly "~" or "~/c1/.../cn".
// It fails Invalid, Unavailable, TooLarge, or with ctx.Err().
func (service *Service) List(ctx context.Context, token string) (Listing, error) {
	relative := "."
	if token != "~" {
		rest, found := strings.CutPrefix(token, "~/")
		if !found || !validPathText(token) {
			return Listing{}, newError(Invalid)
		}
		// An empty component also rejects "~/" and a trailing "/".
		for _, component := range strings.Split(rest, "/") {
			if component == "" || component == "." || component == ".." {
				return Listing{}, newError(Invalid)
			}
		}
		relative = rest
	}
	if err := ctx.Err(); err != nil {
		return Listing{}, err
	}
	home, err := os.OpenRoot(service.home)
	if err != nil {
		return Listing{}, newError(Unavailable)
	}
	defer home.Close()
	// OpenRoot follows symlinks that stay inside Home.
	requested, err := home.OpenRoot(relative)
	if err != nil {
		return Listing{}, newError(Unavailable)
	}
	defer requested.Close()
	directory, err := requested.Open(".")
	if err != nil {
		return Listing{}, newError(Unavailable)
	}
	defer directory.Close()
	if !searchable(home, relative) {
		return Listing{}, newError(Unavailable)
	}

	listing := Listing{Directory: token, Children: []Entry{}}
	if token != "~" {
		listing.ParentDirectory = token[:strings.LastIndexByte(token, '/')]
	}
	textBytes := len(listing.Directory) + len(listing.ParentDirectory)
	seen := map[string]bool{}
	for scanned := 0; ; {
		if err := ctx.Err(); err != nil {
			return Listing{}, err
		}
		batch, readErr := directory.ReadDir(readBatchSize)
		if scanned += len(batch); scanned > maximumScanned {
			return Listing{}, newError(TooLarge)
		}
		for _, entry := range batch {
			if err := ctx.Err(); err != nil {
				return Listing{}, err
			}
			kind, omitted := childKind(home, requested, relative, entry.Name())
			canonical := token + "/" + entry.Name()
			switch {
			case kind == "":
				listing.Omitted = listing.Omitted || omitted
			case !validPathText(canonical) || seen[canonical]:
				listing.Omitted = true
			case len(listing.Children) == maximumChildren:
				return Listing{}, newError(TooLarge)
			default:
				if textBytes += len(canonical); textBytes > maximumPathTextBytes {
					return Listing{}, newError(TooLarge)
				}
				seen[canonical] = true
				listing.Children = append(listing.Children, Entry{Directory: canonical, Kind: kind})
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return Listing{}, newError(Unavailable)
		}
	}
	if err := ctx.Err(); err != nil {
		return Listing{}, err
	}
	// Every child shares the prefix token+"/", so whole-string order is basename order.
	slices.SortFunc(listing.Children, func(a, b Entry) int {
		return cmp.Or(strings.Compare(foldASCII(a.Directory), foldASCII(b.Directory)), strings.Compare(a.Directory, b.Directory))
	})
	return listing, nil
}

// childKind classifies one entry of the requested directory: a kind to list, or ""
// with omitted reporting a directory it cannot show. Files and links to files are skipped.
func childKind(home, requested *os.Root, relative, name string) (kind string, omitted bool) {
	info, err := requested.Lstat(name)
	switch {
	case err != nil:
		return "", true
	case info.Mode()&os.ModeSymlink != 0:
		target, err := requested.Readlink(name)
		if err != nil || filepath.IsAbs(target) {
			return "", true
		}
		// Resolved lexically against the token, so a "../x" link that stays inside Home is listed.
		resolved := filepath.Clean(filepath.Join(relative, target))
		if resolved == ".." || strings.HasPrefix(resolved, "../") {
			return "", true
		}
		info, err = home.Stat(resolved)
		switch {
		case err != nil:
			return "", true
		case !info.IsDir():
			return "", false
		case !searchable(home, resolved):
			return "", true
		}
		return "SymbolicLink", false
	case info.IsDir():
		// os.Root opens every path component, so a listed directory needs read as well as search.
		if !searchable(requested, name) {
			return "", true
		}
		return "Directory", false
	}
	return "", false
}

func searchable(root *os.Root, relative string) bool {
	probe := relative + "/."
	if relative == "." {
		probe = "."
	}
	_, err := root.Stat(probe)
	return err == nil
}

func foldASCII(value string) string {
	folded := []byte(value)
	for index, character := range folded {
		if character >= 'A' && character <= 'Z' {
			folded[index] = character + ('a' - 'A')
		}
	}
	return string(folded)
}

func validPathText(value string) bool {
	if len(value) == 0 || len(value) > maximumPathBytes || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		switch {
		case character <= '\u001f',
			character >= '\u007f' && character <= '\u009f',
			character == '\u061c',
			character >= '\u200e' && character <= '\u200f',
			character == '\u2028',
			character == '\u2029',
			character >= '\u202a' && character <= '\u202e',
			character >= '\u2066' && character <= '\u2069':
			return false
		}
	}
	return true
}

func newError(code ErrorCode) *operationError { return &operationError{code: code} }
