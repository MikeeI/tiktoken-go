package tiktoken

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type BpeLoader interface {
	LoadTiktokenBpe(tiktokenBpeFile string) (map[string]int, error)
}

func readFile(blobpath string) ([]byte, error) {
	if !strings.HasPrefix(blobpath, "http://") && !strings.HasPrefix(blobpath, "https://") {
		file, err := os.Open(blobpath)
		if err != nil {
			return nil, err
		}
		defer func() { _ = file.Close() }()
		return io.ReadAll(file)
	}
	// avoiding blobfile for public files helps avoid auth issues, like MFA prompts
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, blobpath, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"unexpected HTTP status %d (%s) for URL %s",
			resp.StatusCode, resp.Status, blobpath)
	}
	return io.ReadAll(resp.Body)
}

func readFileCached(blobpath string) ([]byte, error) {
	if blobpath == "" {
		return nil, errors.New("blobpath cannot be empty")
	}

	cacheDir := strings.TrimSpace(os.Getenv("TIKTOKEN_CACHE_DIR"))
	if cacheDir == "" {
		cacheDir = strings.TrimSpace(os.Getenv("DATA_GYM_CACHE_DIR"))
	}

	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "data-gym-cache")
	}

	cacheKey := fmt.Sprintf("%x", sha1.Sum([]byte(blobpath)))
	cachePath := filepath.Join(cacheDir, cacheKey)
	if _, err := os.Stat(cachePath); err == nil {
		return os.ReadFile(cachePath)
	}

	contents, err := readFile(blobpath)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	tmpFilename := fmt.Sprintf("%s.%s.tmp", cachePath, uuid.NewString())
	if err := os.WriteFile(tmpFilename, contents, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write temporary file: %w", err)
	}

	if err := os.Rename(tmpFilename, cachePath); err != nil {
		_ = os.Remove(tmpFilename) // Best-effort cleanup after failed rename.
		return nil, fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return contents, nil
}

func loadTiktokenBpe(tiktokenBpeFile string) (map[string]int, error) {
	contents, err := readFileCached(tiktokenBpeFile)
	if err != nil {
		return nil, err
	}

	bpeRanks := make(map[string]int)
	for line := range strings.SplitSeq(string(contents), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		token, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, err
		}
		rank, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}
		bpeRanks[string(token)] = rank
	}
	return bpeRanks, nil
}

type defaultBpeLoader struct{}

func (l *defaultBpeLoader) LoadTiktokenBpe(tiktokenBpeFile string) (map[string]int, error) {
	return loadTiktokenBpe(tiktokenBpeFile)
}

func NewDefaultBpeLoader() BpeLoader {
	return &defaultBpeLoader{}
}
