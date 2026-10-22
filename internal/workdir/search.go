package workdir

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	maximumSearchTerms       = 8
	maximumSearchTermBytes   = 256
	maximumSearchOutputBytes = 64 * 1024
	maximumSearchDirectories = 64
)

type SearchResult struct {
	Directories []string `json:"directories"` // never nil
	Omitted     bool     `json:"omitted"`
}

type searchOutput struct {
	buffer   bytes.Buffer
	tooLarge bool
}

func (output *searchOutput) Write(chunk []byte) (int, error) {
	if len(chunk) > maximumSearchOutputBytes-output.buffer.Len() {
		output.tooLarge = true
		return 0, errors.New("directory search output exceeds limit")
	}
	return output.buffer.Write(chunk)
}

func (service *Service) Search(ctx context.Context, terms []string) (SearchResult, error) {
	if len(terms) == 0 || len(terms) > maximumSearchTerms {
		return SearchResult{}, newError(Invalid)
	}
	termBytes := 0
	for _, term := range terms {
		termBytes += len(term)
		if termBytes > maximumSearchTermBytes || !validPathText(term) {
			return SearchResult{}, newError(Invalid)
		}
		for _, character := range term {
			if unicode.IsSpace(character) {
				return SearchResult{}, newError(Invalid)
			}
		}
	}
	if service.zoxidePath == "" {
		return SearchResult{}, newError(Unavailable)
	}
	if ctx.Err() != nil {
		return SearchResult{}, ctx.Err()
	}
	// Reserve the final 100 ms of the deadline for closing inherited pipes.
	queryContext, cancel := context.WithTimeout(ctx, 1900*time.Millisecond)
	defer cancel()
	arguments := append([]string{"query", "--list", "--"}, terms...)
	command := exec.CommandContext(queryContext, service.zoxidePath, arguments...)
	command.WaitDelay = 100 * time.Millisecond
	var output searchOutput
	command.Stdout = &output
	command.Stderr = nil
	err := command.Run()
	if ctx.Err() != nil {
		return SearchResult{}, ctx.Err()
	}
	if output.tooLarge {
		return SearchResult{}, newError(TooLarge)
	}
	if err != nil || queryContext.Err() != nil {
		return SearchResult{}, newError(Unavailable)
	}
	result := SearchResult{Directories: []string{}}
	seen := map[string]bool{}
	pathBytes := 0
	for _, line := range strings.Split(strings.TrimSuffix(output.buffer.String(), "\n"), "\n") {
		if line == "" && output.buffer.Len() == 0 {
			break
		}
		// zoxide lists absolute paths: a "~" line is omitted, never expanded under Home.
		path, err := service.ParseStart(line)
		if !filepath.IsAbs(line) || err != nil || CheckStart(path) != nil || seen[path] ||
			len(result.Directories) == maximumSearchDirectories || pathBytes+len(path) > maximumPathTextBytes {
			result.Omitted = true
			continue
		}
		seen[path] = true
		pathBytes += len(path)
		result.Directories = append(result.Directories, path)
	}
	return result, nil
}
