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
	Directories []string
	Omitted     bool
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
	result := SearchResult{Directories: make([]string, 0)}
	seen := make(map[string]struct{})
	pathBytes := 0
	for _, path := range strings.Split(strings.TrimSuffix(output.buffer.String(), "\n"), "\n") {
		if path == "" && output.buffer.Len() == 0 {
			break
		}
		if !filepath.IsAbs(path) {
			result.Omitted = true
			continue
		}
		candidate, err := service.ParseCandidate(path)
		if err != nil {
			result.Omitted = true
			continue
		}
		directory, err := service.ValidateStart(candidate)
		if err != nil {
			result.Omitted = true
			continue
		}
		canonical := directory.String()
		if _, duplicate := seen[canonical]; duplicate {
			result.Omitted = true
			continue
		}
		if len(result.Directories) == maximumSearchDirectories || pathBytes+len(canonical) > maximumPathTextBytes {
			result.Omitted = true
			continue
		}
		seen[canonical] = struct{}{}
		pathBytes += len(canonical)
		result.Directories = append(result.Directories, canonical)
	}
	return result, nil
}
