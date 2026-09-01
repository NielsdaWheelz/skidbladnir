package workdir

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
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

type WorkingDirectoryCandidate struct {
	path string
}

type WorkingDirectory struct {
	path string
}

func (directory WorkingDirectory) String() string { return directory.path }

type HomeDirectory struct {
	canonical string
	relative  string
}

func (directory HomeDirectory) String() string { return directory.canonical }

type ParentDirectory struct {
	directory HomeDirectory
	present   bool
}

func (parent ParentDirectory) Value() (HomeDirectory, bool) {
	return parent.directory, parent.present
}

type EntryKind string

const (
	Directory    EntryKind = "Directory"
	SymbolicLink EntryKind = "SymbolicLink"
)

type Entry struct {
	directory HomeDirectory
	kind      EntryKind
}

func (entry Entry) Directory() HomeDirectory { return entry.directory }

func (entry Entry) Kind() EntryKind { return entry.kind }

type Omissions bool

const (
	None    Omissions = false
	Present Omissions = true
)

type Listing struct {
	directory HomeDirectory
	parent    ParentDirectory
	children  []Entry
	omissions Omissions
}

func (listing Listing) Directory() HomeDirectory { return listing.directory }

func (listing Listing) Parent() ParentDirectory { return listing.parent }

func (listing Listing) Children() []Entry {
	return append([]Entry(nil), listing.children...)
}

func (listing Listing) Omissions() Omissions { return listing.omissions }

type Service struct {
	home string
}

func New(home string) (*Service, error) {
	if !validPathText(home) || !filepath.IsAbs(home) || filepath.Clean(home) != home {
		return nil, newError(Invalid)
	}
	if !searchableDirectory(home) {
		return nil, newError(Unavailable)
	}
	return &Service{home: home}, nil
}

func (service *Service) ParseCandidate(value string) (WorkingDirectoryCandidate, error) {
	if !validPathText(value) {
		return WorkingDirectoryCandidate{}, newError(Invalid)
	}
	expanded := value
	switch {
	case value == "~":
		expanded = service.home
	case strings.HasPrefix(value, "~/"):
		expanded = service.home + string(filepath.Separator) + value[2:]
	case !filepath.IsAbs(value):
		return WorkingDirectoryCandidate{}, newError(Invalid)
	}
	normalized := filepath.Clean(expanded)
	if !validPathText(normalized) || !filepath.IsAbs(normalized) {
		return WorkingDirectoryCandidate{}, newError(Invalid)
	}
	return WorkingDirectoryCandidate{path: normalized}, nil
}

func (service *Service) ParseBrowseDirectory(value string) (HomeDirectory, error) {
	if !validPathText(value) {
		return HomeDirectory{}, newError(Invalid)
	}
	if value == "~" {
		return HomeDirectory{canonical: "~", relative: "."}, nil
	}
	if !strings.HasPrefix(value, "~/") || strings.HasSuffix(value, "/") {
		return HomeDirectory{}, newError(Invalid)
	}
	relative := value[2:]
	for _, component := range strings.Split(relative, "/") {
		if component == "" || component == "." || component == ".." {
			return HomeDirectory{}, newError(Invalid)
		}
	}
	return HomeDirectory{canonical: value, relative: relative}, nil
}

func (service *Service) ValidateStart(candidate WorkingDirectoryCandidate) (WorkingDirectory, error) {
	if candidate.path == "" {
		panic("working directory candidate is not initialized") // justify-defect: only ParseCandidate can construct this opaque value.
	}
	if !searchableDirectory(candidate.path) {
		return WorkingDirectory{}, newError(Unavailable)
	}
	return WorkingDirectory{path: candidate.path}, nil
}

func (service *Service) List(ctx context.Context, directory HomeDirectory) (Listing, error) {
	if err := ctx.Err(); err != nil {
		return Listing{}, err
	}
	if directory.canonical == "" || directory.relative == "" {
		panic("Home directory token is not initialized") // justify-defect: only ParseBrowseDirectory can construct this opaque value.
	}
	root, err := os.OpenRoot(service.home)
	if err != nil {
		return Listing{}, newError(Unavailable)
	}
	requestedRoot, err := root.OpenRoot(directory.relative)
	if err != nil {
		closeRootErr := root.Close()
		if closeRootErr != nil {
			return Listing{}, newError(Unavailable)
		}
		return Listing{}, newError(Unavailable)
	}
	opened, err := requestedRoot.Open(".")
	if err != nil {
		closeRequestedErr := requestedRoot.Close()
		closeRootErr := root.Close()
		if closeRequestedErr != nil || closeRootErr != nil {
			return Listing{}, newError(Unavailable)
		}
		return Listing{}, newError(Unavailable)
	}
	listing, listErr := listOpened(ctx, root, requestedRoot, opened, directory)
	closeDirectoryErr := opened.Close()
	closeRequestedErr := requestedRoot.Close()
	closeRootErr := root.Close()
	if listErr != nil {
		if closeDirectoryErr != nil || closeRequestedErr != nil || closeRootErr != nil {
			return Listing{}, errors.Join(listErr, newError(Unavailable))
		}
		return Listing{}, listErr
	}
	if closeDirectoryErr != nil || closeRequestedErr != nil || closeRootErr != nil {
		return Listing{}, newError(Unavailable)
	}
	return listing, nil
}

func listOpened(
	ctx context.Context,
	homeRoot *os.Root,
	requestedRoot *os.Root,
	opened *os.File,
	directory HomeDirectory,
) (Listing, error) {
	if !rootSearchable(homeRoot, directory.relative) ||
		!sameRequestedDirectory(homeRoot, requestedRoot, directory.relative) {
		return Listing{}, newError(Unavailable)
	}
	parent := parentOf(directory)
	pathTextBytes := len(directory.canonical)
	if parentDirectory, present := parent.Value(); present {
		pathTextBytes += len(parentDirectory.canonical)
	}
	children := make([]Entry, 0)
	seen := make(map[string]struct{})
	omissions := None
	scanned := 0
	for {
		if err := ctx.Err(); err != nil {
			return Listing{}, err
		}
		batch, err := opened.ReadDir(readBatchSize)
		scanned += len(batch)
		if scanned > maximumScanned {
			return Listing{}, newError(TooLarge)
		}
		if !sameRequestedDirectory(homeRoot, requestedRoot, directory.relative) {
			return Listing{}, newError(Unavailable)
		}
		for _, candidate := range batch {
			if err := ctx.Err(); err != nil {
				return Listing{}, err
			}
			entry, include, omitted := projectEntry(
				homeRoot,
				requestedRoot,
				directory,
				candidate.Name(),
			)
			if omitted {
				omissions = Present
			}
			if !include {
				continue
			}
			canonical := entry.directory.canonical
			if _, duplicate := seen[canonical]; duplicate {
				omissions = Present
				continue
			}
			if len(children) == maximumChildren {
				return Listing{}, newError(TooLarge)
			}
			pathTextBytes += len(entry.directory.canonical)
			if pathTextBytes > maximumPathTextBytes {
				return Listing{}, newError(TooLarge)
			}
			seen[canonical] = struct{}{}
			children = append(children, entry)
		}
		if !sameRequestedDirectory(homeRoot, requestedRoot, directory.relative) {
			return Listing{}, newError(Unavailable)
		}
		switch {
		case errors.Is(err, io.EOF):
			if err := ctx.Err(); err != nil {
				return Listing{}, err
			}
			sortEntries(children)
			return Listing{directory: directory, parent: parent, children: children, omissions: omissions}, nil
		case err != nil:
			return Listing{}, newError(Unavailable)
		}
	}
}

func projectEntry(
	homeRoot *os.Root,
	requestedRoot *os.Root,
	parent HomeDirectory,
	name string,
) (Entry, bool, bool) {
	info, err := requestedRoot.Lstat(name)
	if err != nil {
		return Entry{}, false, true
	}
	kind := Directory
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		kind = SymbolicLink
		target, readErr := requestedRoot.Readlink(name)
		if readErr != nil || filepath.IsAbs(target) {
			return Entry{}, false, true
		}
		targetRelative := filepath.Clean(filepath.Join(parent.relative, target))
		if targetRelative == ".." || strings.HasPrefix(targetRelative, ".."+string(filepath.Separator)) {
			return Entry{}, false, true
		}
		resolved, resolveErr := homeRoot.Stat(targetRelative)
		if resolveErr != nil {
			return Entry{}, false, true
		}
		if !resolved.IsDir() {
			return Entry{}, false, false
		}
		if !rootSearchable(homeRoot, targetRelative) {
			return Entry{}, false, true
		}
		after, afterErr := requestedRoot.Lstat(name)
		afterTarget, afterReadErr := requestedRoot.Readlink(name)
		currentTarget, currentTargetErr := homeRoot.Stat(targetRelative)
		if afterErr != nil || afterReadErr != nil || currentTargetErr != nil ||
			!os.SameFile(info, after) || target != afterTarget ||
			!os.SameFile(resolved, currentTarget) {
			return Entry{}, false, true
		}
	case info.IsDir():
		if !rootSearchable(requestedRoot, name) {
			return Entry{}, false, true
		}
		after, afterErr := requestedRoot.Lstat(name)
		if afterErr != nil || !os.SameFile(info, after) || !after.IsDir() {
			return Entry{}, false, true
		}
	default:
		return Entry{}, false, false
	}
	canonical := childCanonical(parent, name)
	if !validPathText(canonical) || strings.ContainsRune(name, '/') || name == "." || name == ".." {
		return Entry{}, false, true
	}
	return Entry{
		directory: HomeDirectory{canonical: canonical, relative: childRelative(parent, name)},
		kind:      kind,
	}, true, false
}

func sameRequestedDirectory(homeRoot, requestedRoot *os.Root, relative string) bool {
	pinned, pinnedErr := requestedRoot.Stat(".")
	current, currentErr := homeRoot.Stat(relative)
	return pinnedErr == nil && currentErr == nil && os.SameFile(pinned, current)
}

func parentOf(directory HomeDirectory) ParentDirectory {
	if directory.canonical == "~" {
		return ParentDirectory{}
	}
	separator := strings.LastIndexByte(directory.canonical, '/')
	if separator == 1 {
		return ParentDirectory{directory: HomeDirectory{canonical: "~", relative: "."}, present: true}
	}
	canonical := directory.canonical[:separator]
	return ParentDirectory{directory: HomeDirectory{canonical: canonical, relative: canonical[2:]}, present: true}
}

func childRelative(parent HomeDirectory, name string) string {
	if parent.relative == "." {
		return name
	}
	return parent.relative + "/" + name
}

func childCanonical(parent HomeDirectory, name string) string {
	if parent.canonical == "~" {
		return "~/" + name
	}
	return parent.canonical + "/" + name
}

func rootSearchable(root *os.Root, relative string) bool {
	probe := relative + "/."
	if relative == "." {
		probe = "."
	}
	_, err := root.Stat(probe)
	return err == nil
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(left, right int) bool {
		leftName := entryBase(entries[left])
		rightName := entryBase(entries[right])
		leftFolded := foldASCII(leftName)
		rightFolded := foldASCII(rightName)
		if leftFolded != rightFolded {
			return leftFolded < rightFolded
		}
		return leftName < rightName
	})
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

func entryBase(entry Entry) string {
	canonical := entry.directory.canonical
	return canonical[strings.LastIndexByte(canonical, '/')+1:]
}

func searchableDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	return syscall.Access(path, 1) == nil
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
