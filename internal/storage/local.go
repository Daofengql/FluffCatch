package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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

	target := filepath.Join(store.root, filepath.FromSlash(cleanKey))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return StoredObject{}, fmt.Errorf("create object directory: %w", err)
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

	file, err := os.Open(filepath.Join(store.root, filepath.FromSlash(cleanKey)))
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

	if err := os.Remove(filepath.Join(store.root, filepath.FromSlash(cleanKey))); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}

func (store *LocalStore) PublicURL(key string) string {
	escaped := strings.ReplaceAll(url.PathEscape(key), "%2F", "/")
	return store.publicPrefix + "/" + escaped
}

func cleanObjectKey(key string) (string, error) {
	cleanKey := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(key)), "/")
	if cleanKey == "." || cleanKey == "" {
		return "", fmt.Errorf("object key is required")
	}

	if strings.HasPrefix(cleanKey, "../") || cleanKey == ".." || strings.Contains(cleanKey, "/../") {
		return "", fmt.Errorf("object key cannot traverse directories")
	}

	return cleanKey, nil
}
