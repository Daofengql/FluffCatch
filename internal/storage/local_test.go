package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorePutAndPublicURL(t *testing.T) {
	store := NewLocalStore(t.TempDir(), "/media")

	stored, err := store.Put(context.Background(), Object{
		Key:         "events/sample/cover image.jpg",
		Content:     strings.NewReader("hello"),
		ContentType: "text/plain",
		Size:        5,
	})
	if err != nil {
		t.Fatalf("Put() returned error: %v", err)
	}

	if stored.Key != "events/sample/cover image.jpg" {
		t.Fatalf("unexpected key %q", stored.Key)
	}

	if stored.URL != "/media/events/sample/cover%20image.jpg" {
		t.Fatalf("unexpected public URL %q", stored.URL)
	}
}

func TestLocalStoreRejectsTraversal(t *testing.T) {
	store := NewLocalStore(t.TempDir(), "/media")

	cases := []string{
		"../secret.txt",
		"..\\secret.txt",
		"events/../secret.txt",
		"/etc/passwd",
		"C:/Windows/win.ini",
		"C:\\Windows\\win.ini",
		"events/image.jpg:ads",
	}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			_, err := store.Put(context.Background(), Object{
				Key:     key,
				Content: strings.NewReader("nope"),
			})
			if err == nil {
				t.Fatal("expected unsafe key error")
			}
		})
	}
}

func TestLocalFilePathStaysWithinRoot(t *testing.T) {
	root := t.TempDir()

	cleanKey, target, err := LocalFilePath(root, "events/sample/image.jpg")
	if err != nil {
		t.Fatalf("LocalFilePath() returned error: %v", err)
	}
	if cleanKey != "events/sample/image.jpg" {
		t.Fatalf("unexpected clean key %q", cleanKey)
	}

	rel, err := filepath.Rel(root, target)
	if err != nil {
		t.Fatalf("relative path: %v", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		t.Fatalf("target escaped root: %q", target)
	}
}
