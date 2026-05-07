package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type LocalStore struct {
	root         string
	publicPrefix string
}

func NewLocalStore(root string, publicPrefix string) *LocalStore {
	return &LocalStore{
		root:         root,
		publicPrefix: strings.TrimRight(publicPrefix, "/"),
	}
}

func (store *LocalStore) Put(ctx context.Context, object Object) (StoredObject, error) {
	_ = ctx

	cleanKey, err := cleanObjectKey(object.Key)
	if err != nil {
		return StoredObject{}, err
	}

	_, target, err := LocalFilePath(store.root, cleanKey)
	if err != nil {
		return StoredObject{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return StoredObject{}, fmt.Errorf("create object directory: %w", err)
	}
	if err := ensureResolvedPathWithinRoot(store.root, filepath.Dir(target)); err != nil {
		return StoredObject{}, err
	}

	file, err := os.Create(target)
	if err != nil {
		return StoredObject{}, fmt.Errorf("create object: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, object.Content); err != nil {
		return StoredObject{}, fmt.Errorf("write object: %w", err)
	}

	return StoredObject{
		Key: cleanKey,
		URL: store.PublicURL(cleanKey),
	}, nil
}

func (store *LocalStore) Get(ctx context.Context, key string) (ObjectReader, error) {
	_ = ctx

	cleanKey, err := cleanObjectKey(key)
	if err != nil {
		return ObjectReader{}, err
	}

	_, target, err := LocalFilePath(store.root, cleanKey)
	if err != nil {
		return ObjectReader{}, err
	}
	if err := ensureResolvedPathWithinRoot(store.root, target); err != nil {
		return ObjectReader{}, err
	}

	file, err := os.Open(target)
	if err != nil {
		if os.IsNotExist(err) {
			return ObjectReader{}, fmt.Errorf("object not found")
		}
		return ObjectReader{}, fmt.Errorf("open object: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return ObjectReader{}, fmt.Errorf("stat object: %w", err)
	}

	head := make([]byte, 512)
	n, _ := file.Read(head)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return ObjectReader{}, fmt.Errorf("seek object: %w", err)
	}

	return ObjectReader{
		Content:       file,
		ContentType:   http.DetectContentType(head[:n]),
		ContentLength: info.Size(),
	}, nil
}

func (store *LocalStore) Delete(ctx context.Context, key string) error {
	_ = ctx

	cleanKey, err := cleanObjectKey(key)
	if err != nil {
		return err
	}

	_, target, err := LocalFilePath(store.root, cleanKey)
	if err != nil {
		return err
	}
	if err := ensureResolvedPathWithinRoot(store.root, target); err != nil {
		return err
	}

	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}

func (store *LocalStore) PublicURL(key string) string {
	escaped := strings.ReplaceAll(url.PathEscape(key), "%2F", "/")
	return store.publicPrefix + "/" + escaped
}

func LocalFilePath(root string, key string) (string, string, error) {
	cleanKey, err := cleanObjectKey(key)
	if err != nil {
		return "", "", err
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve storage root: %w", err)
	}
	targetAbs, err := filepath.Abs(filepath.Join(rootAbs, filepath.FromSlash(cleanKey)))
	if err != nil {
		return "", "", fmt.Errorf("resolve object path: %w", err)
	}
	if err := ensurePathWithinRoot(rootAbs, targetAbs); err != nil {
		return "", "", err
	}

	return cleanKey, targetAbs, nil
}

func cleanObjectKey(key string) (string, error) {
	if strings.ContainsRune(key, '\x00') {
		return "", fmt.Errorf("object key contains invalid bytes")
	}

	key = strings.ReplaceAll(key, "\\", "/")
	if strings.HasPrefix(key, "/") || hasWindowsVolume(key) {
		return "", fmt.Errorf("object key must be relative")
	}

	parts := []string{}
	for _, part := range strings.Split(key, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", fmt.Errorf("object key cannot traverse directories")
		}
		if strings.Contains(part, ":") {
			return "", fmt.Errorf("object key contains invalid path segment")
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("object key is required")
	}

	cleanKey := path.Clean(strings.Join(parts, "/"))
	if cleanKey == "." || cleanKey == "" || strings.HasPrefix(cleanKey, "../") || cleanKey == ".." || strings.Contains(cleanKey, "/../") {
		return "", fmt.Errorf("object key cannot traverse directories")
	}

	return cleanKey, nil
}

func hasWindowsVolume(key string) bool {
	return len(key) >= 2 && ((key[0] >= 'a' && key[0] <= 'z') || (key[0] >= 'A' && key[0] <= 'Z')) && key[1] == ':'
}

func ensurePathWithinRoot(root string, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("verify object path: %w", err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("object path escapes storage root")
	}
	return nil
}

func ensureResolvedPathWithinRoot(root string, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve storage root: %w", err)
	}
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil
	}
	targetResolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return nil
	}
	if err := ensurePathWithinRoot(rootResolved, targetResolved); err != nil {
		return fmt.Errorf("object path escapes storage root through symlink")
	}
	return nil
}
