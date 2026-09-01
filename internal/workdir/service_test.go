package workdir

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestServiceListsOneSafeHomeLevelAndReportsOmissions(t *testing.T) {
	home := t.TempDir()
	mustMakeDirectory(t, filepath.Join(home, ".hidden"))
	mustMakeDirectory(t, filepath.Join(home, "alpha"))
	mustMakeDirectory(t, filepath.Join(home, "Bravo"))
	mustWriteFile(t, filepath.Join(home, "ordinary-file"))
	mustMakeDirectory(t, filepath.Join(home, "target"))
	mustMakeDirectory(t, filepath.Join(home, "target", "nested"))
	mustMakeDirectory(t, filepath.Join(home, "Ångström"))
	mustMakeDirectory(t, filepath.Join(home, "iz"))
	mustMakeDirectory(t, filepath.Join(home, "İa"))
	mustMakeSymlink(t, "target", filepath.Join(home, "inside-link"))
	outside := t.TempDir()
	mustMakeDirectory(t, filepath.Join(outside, "external"))
	externalTarget, err := filepath.Rel(home, filepath.Join(outside, "external"))
	if err != nil {
		t.Fatalf("construct external relative symlink fixture: %v", err)
	}
	mustMakeSymlink(t, externalTarget, filepath.Join(home, "outside-link"))
	mustMakeSymlink(t, filepath.Join(home, "target"), filepath.Join(home, "absolute-link"))
	mustMakeSymlink(t, "missing", filepath.Join(home, "broken-link"))
	mustMakeSymlink(t, "ordinary-file", filepath.Join(home, "file-link"))
	mustMakeDirectory(t, filepath.Join(home, "unsafe\nname"))
	mustMakeDirectory(t, filepath.Join(home, "unsafe\u061cname"))

	service, err := New(home)
	if err != nil {
		t.Fatalf("construct workdir service: %v", err)
	}
	directory, err := service.ParseBrowseDirectory("~")
	if err != nil {
		t.Fatalf("parse canonical Home directory: %v", err)
	}
	listing, err := service.List(context.Background(), directory)
	if err != nil {
		t.Fatalf("list canonical Home directory: %v", err)
	}

	children := listing.Children()
	if len(children) != 8 {
		t.Fatalf("listed child count = %d, want 8", len(children))
	}
	wantDirectories := []string{"~/.hidden", "~/alpha", "~/Bravo", "~/inside-link", "~/iz", "~/target", "~/Ångström", "~/İa"}
	wantKinds := []EntryKind{Directory, Directory, Directory, SymbolicLink, Directory, Directory, Directory, Directory}
	for index, child := range children {
		if child.Directory().String() != wantDirectories[index] || child.Kind() != wantKinds[index] {
			t.Fatalf("listed child %d did not match the canonical ordered projection", index)
		}
	}
	if listing.Directory().String() != "~" {
		t.Fatal("listing did not retain the requested canonical directory")
	}
	if _, present := listing.Parent().Value(); present {
		t.Fatal("Home listing unexpectedly had a parent")
	}
	if listing.Omissions() != Present {
		t.Fatal("unsafe or broken candidates did not set the omission state")
	}
	linkedDirectory, err := service.ParseBrowseDirectory("~/inside-link")
	if err != nil {
		t.Fatalf("parse internal symlink token: %v", err)
	}
	linkedListing, err := service.List(context.Background(), linkedDirectory)
	if err != nil {
		t.Fatalf("traverse internal symlink token: %v", err)
	}
	linkedParent, linkedParentPresent := linkedListing.Parent().Value()
	linkedChildren := linkedListing.Children()
	if linkedListing.Directory().String() != "~/inside-link" ||
		!linkedParentPresent || linkedParent.String() != "~" ||
		len(linkedChildren) != 1 ||
		linkedChildren[0].Directory().String() != "~/inside-link/nested" ||
		linkedChildren[0].Kind() != Directory {
		t.Fatal("internal relative symlink was marked but not safely traversable")
	}
}

func TestServiceIgnoresSymlinksToFilesWithoutReportingOmission(t *testing.T) {
	home := t.TempDir()
	mustWriteFile(t, filepath.Join(home, "ordinary-file"))
	mustMakeSymlink(t, "ordinary-file", filepath.Join(home, "file-link"))
	service, directory := listingFixtureForHome(t, home)

	listing, err := service.List(context.Background(), directory)
	if err != nil {
		t.Fatalf("list Home containing a symlink to a file: %v", err)
	}
	if len(listing.Children()) != 0 || listing.Omissions() != None {
		t.Fatal("a symlink to an ordinary file was reported as an omitted folder")
	}
}

func TestListRejectsRequestedDirectoryIdentityDrift(t *testing.T) {
	home := t.TempDir()
	requestedPath := filepath.Join(home, "requested")
	mustMakeDirectory(t, requestedPath)
	mustMakeDirectory(t, filepath.Join(requestedPath, "old-child"))
	service, err := New(home)
	if err != nil {
		t.Fatalf("construct workdir service: %v", err)
	}
	directory, err := service.ParseBrowseDirectory("~/requested")
	if err != nil {
		t.Fatalf("parse requested directory: %v", err)
	}
	root, err := os.OpenRoot(service.home)
	if err != nil {
		t.Fatalf("open Home root: %v", err)
	}
	defer root.Close()
	requestedRoot, err := root.OpenRoot(directory.relative)
	if err != nil {
		t.Fatalf("open requested root: %v", err)
	}
	defer requestedRoot.Close()
	opened, err := requestedRoot.Open(".")
	if err != nil {
		t.Fatalf("open requested directory: %v", err)
	}
	defer opened.Close()
	if err := os.Rename(requestedPath, filepath.Join(home, "moved")); err != nil {
		t.Fatalf("move requested directory during listing: %v", err)
	}
	mustMakeDirectory(t, requestedPath)

	if _, err := listOpened(
		context.Background(),
		root,
		requestedRoot,
		opened,
		directory,
	); !isCode(err, Unavailable) {
		t.Fatal("listing mixed a pinned directory stream with a replacement pathname")
	}
}

func TestServiceOwnsCreateAndBrowseGrammarAndRevalidatesStart(t *testing.T) {
	home := t.TempDir()
	nested := filepath.Join(home, "nested")
	mustMakeDirectory(t, nested)
	service, err := New(home)
	if err != nil {
		t.Fatalf("construct workdir service: %v", err)
	}

	for _, input := range []string{"~", "~/nested", nested} {
		candidate, parseErr := service.ParseCandidate(input)
		if parseErr != nil {
			t.Fatal("valid create candidate was rejected")
		}
		validated, validateErr := service.ValidateStart(candidate)
		if validateErr != nil || !filepath.IsAbs(validated.String()) {
			t.Fatal("valid create candidate did not become an absolute working directory")
		}
	}
	for _, input := range []string{
		"",
		"relative",
		"~someone",
		"~/bad\x00value",
		"~/bad\u0085value",
		"~/bad\u061cvalue",
		"~/bad\u200evalue",
		"~/bad\u200fvalue",
		"~/bad\u2028value",
		"~/bad\u2029value",
		"~/bad\u202evalue",
		"~/bad\u2066value",
		string([]byte{0xff}),
		"/" + strings.Repeat("a", maximumPathBytes),
	} {
		if _, parseErr := service.ParseCandidate(input); !isCode(parseErr, Invalid) {
			t.Fatal("invalid create candidate did not return Invalid")
		}
	}
	if _, parseErr := service.ParseCandidate("~/" + strings.Repeat("a", maximumPathBytes-16)); !isCode(parseErr, Invalid) {
		t.Fatal("create candidate whose expansion exceeded the bound did not return Invalid")
	}
	for _, input := range []string{"", "/absolute", "relative", "~/", "~//nested", "~/./nested", "~/../nested"} {
		if _, parseErr := service.ParseBrowseDirectory(input); !isCode(parseErr, Invalid) {
			t.Fatal("noncanonical browse directory did not return Invalid")
		}
	}
	missing, parseErr := service.ParseCandidate(filepath.Join(home, "missing"))
	if parseErr != nil {
		t.Fatal("missing absolute create candidate was not representable")
	}
	if _, validateErr := service.ValidateStart(missing); !isCode(validateErr, Unavailable) {
		t.Fatal("missing create candidate did not return Unavailable")
	}
	file := filepath.Join(home, "file")
	mustWriteFile(t, file)
	fileCandidate, parseErr := service.ParseCandidate(file)
	if parseErr != nil {
		t.Fatal("absolute file candidate was not representable")
	}
	if _, validateErr := service.ValidateStart(fileCandidate); !isCode(validateErr, Unavailable) {
		t.Fatal("file create candidate did not return Unavailable")
	}
}

func TestNewRequiresCanonicalSearchableAbsoluteHome(t *testing.T) {
	home := t.TempDir()
	if _, err := New(home); err != nil {
		t.Fatalf("canonical searchable Home was rejected: %v", err)
	}
	if _, err := New(home + string(filepath.Separator) + "."); !isCode(err, Invalid) {
		t.Fatal("non-clean Home did not return Invalid")
	}
	if _, err := New("relative"); !isCode(err, Invalid) {
		t.Fatal("relative Home did not return Invalid")
	}
	if _, err := New(filepath.Join(home, "missing")); !isCode(err, Unavailable) {
		t.Fatal("missing Home did not return Unavailable")
	}
}

func TestServiceProjectsCanonicalParentsAndDirectChildren(t *testing.T) {
	home := t.TempDir()
	mustMakeDirectory(t, filepath.Join(home, "level"))
	mustMakeDirectory(t, filepath.Join(home, "level", "child"))
	mustWriteFile(t, filepath.Join(home, "level", "file"))
	service, err := New(home)
	if err != nil {
		t.Fatalf("construct workdir service: %v", err)
	}
	directory, err := service.ParseBrowseDirectory("~/level")
	if err != nil {
		t.Fatalf("parse nested browse directory: %v", err)
	}
	listing, err := service.List(context.Background(), directory)
	if err != nil {
		t.Fatalf("list nested browse directory: %v", err)
	}
	parent, present := listing.Parent().Value()
	children := listing.Children()
	if listing.Directory().String() != "~/level" || !present || parent.String() != "~" || len(children) != 1 || children[0].Directory().String() != "~/level/child" || children[0].Kind() != Directory || listing.Omissions() != None {
		t.Fatal("nested listing did not preserve its canonical direct relationship")
	}
	children[0] = Entry{}
	if len(listing.Children()) != 1 || listing.Children()[0].Directory().String() != "~/level/child" {
		t.Fatal("listing exposed its owned child slice")
	}
}

func TestServiceTreatsPermissionAndRequestedDirectoryFailuresAsUnavailable(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("service hosts are Darwin and Linux")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the service-UID permission proof")
	}
	home := t.TempDir()
	locked := filepath.Join(home, "locked")
	mustMakeDirectory(t, locked)
	service, err := New(home)
	if err != nil {
		t.Fatalf("construct workdir service: %v", err)
	}
	if err := os.Chmod(locked, 0o600); err != nil {
		t.Fatalf("remove search permission: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0o700); err != nil && !os.IsNotExist(err) {
			t.Errorf("restore directory fixture permission: %v", err)
		}
	})
	homeDirectory, err := service.ParseBrowseDirectory("~")
	if err != nil {
		t.Fatalf("parse Home directory: %v", err)
	}
	listing, err := service.List(context.Background(), homeDirectory)
	if err != nil || len(listing.Children()) != 0 || listing.Omissions() != Present {
		t.Fatal("unsearchable direct child was not omitted")
	}
	candidate, err := service.ParseCandidate("~/locked")
	if err != nil {
		t.Fatalf("parse unsearchable create candidate: %v", err)
	}
	if _, err := service.ValidateStart(candidate); !isCode(err, Unavailable) {
		t.Fatal("unsearchable start did not return Unavailable")
	}
	requested, err := service.ParseBrowseDirectory("~/locked")
	if err != nil {
		t.Fatalf("parse unsearchable browse directory: %v", err)
	}
	if _, err := service.List(context.Background(), requested); !isCode(err, Unavailable) {
		t.Fatal("unsearchable requested directory did not return Unavailable")
	}
	if err := os.Chmod(home, 0o600); err != nil {
		t.Fatalf("remove Home search permission: %v", err)
	}
	_, homeListErr := service.List(context.Background(), homeDirectory)
	if err := os.Chmod(home, 0o700); err != nil {
		t.Fatalf("restore Home search permission: %v", err)
	}
	if !isCode(homeListErr, Unavailable) {
		t.Fatal("unsearchable Home did not return Unavailable")
	}
}

func TestServiceReturnsCancellationAndRejectsBoundedListings(t *testing.T) {
	t.Run("cancellation", func(t *testing.T) {
		home := t.TempDir()
		mustMakeDirectory(t, filepath.Join(home, "first"))
		service, directory := listingFixtureForHome(t, home)
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		if _, listErr := service.List(canceled, directory); !errors.Is(listErr, context.Canceled) {
			t.Fatal("listing did not preserve caller cancellation")
		}
	})
	t.Run("child bound", func(t *testing.T) {
		home := t.TempDir()
		for index := 0; index < maximumChildren+1; index++ {
			mustMakeDirectory(t, filepath.Join(home, boundedName(index)))
		}
		service, directory := listingFixtureForHome(t, home)
		if _, listErr := service.List(context.Background(), directory); !isCode(listErr, TooLarge) {
			t.Fatal("listing beyond the child bound did not return TooLarge")
		}
		if err := os.Remove(filepath.Join(home, boundedName(maximumChildren))); err != nil {
			t.Fatalf("reduce child-bound fixture: %v", err)
		}
		listing, listErr := service.List(context.Background(), directory)
		if listErr != nil || len(listing.Children()) != maximumChildren {
			t.Fatal("listing at the child bound was not accepted")
		}
	})
	t.Run("scan bound", func(t *testing.T) {
		home := t.TempDir()
		for index := 0; index < maximumScanned+1; index++ {
			mustWriteFile(t, filepath.Join(home, fmt.Sprintf("entry-%04d", index)))
		}
		service, directory := listingFixtureForHome(t, home)
		if _, listErr := service.List(context.Background(), directory); !isCode(listErr, TooLarge) {
			t.Fatal("listing beyond the scan bound did not return TooLarge")
		}
	})
	t.Run("path-text bound", func(t *testing.T) {
		home := t.TempDir()
		for index := 0; index < 225; index++ {
			name := fmt.Sprintf("%03d-%s", index, strings.Repeat("x", 140))
			mustMakeDirectory(t, filepath.Join(home, name))
		}
		service, directory := listingFixtureForHome(t, home)
		if _, listErr := service.List(context.Background(), directory); !isCode(listErr, TooLarge) {
			t.Fatal("listing beyond the returned path-text bound did not return TooLarge")
		}
	})
}

func listingFixtureForHome(t *testing.T, home string) (*Service, HomeDirectory) {
	t.Helper()
	service, err := New(home)
	if err != nil {
		t.Fatalf("construct workdir service: %v", err)
	}
	directory, err := service.ParseBrowseDirectory("~")
	if err != nil {
		t.Fatalf("parse canonical Home directory: %v", err)
	}
	return service, directory
}

func isCode(err error, code ErrorCode) bool {
	observed, classified := ErrorCodeOf(err)
	return classified && observed == code
}

func boundedName(index int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	return "bounded-" + string(alphabet[index/26]) + string(alphabet[index%26])
}

func mustMakeDirectory(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create directory fixture: %v", err)
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("create file fixture: %v", err)
	}
}

func mustMakeSymlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("create symlink fixture: %v", err)
	}
}
