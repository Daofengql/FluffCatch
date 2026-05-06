package storage

import (
	"context"
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

	_, err := store.Put(context.Background(), Object{
		Key:     "../secret.txt",
		Content: strings.NewReader("nope"),
	})
	if err == nil {
		t.Fatal("expected traversal error")
	}
}
