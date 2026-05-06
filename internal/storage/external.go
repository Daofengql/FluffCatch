package storage

import (
	"context"
	"fmt"
)

type ExternalStore struct {
	publicBaseURL string
}

func NewExternalStore(publicBaseURL string) *ExternalStore {
	return &ExternalStore{publicBaseURL: publicBaseURL}
}

func (store *ExternalStore) Put(ctx context.Context, object Object) (StoredObject, error) {
	_ = ctx
	_ = object
	return StoredObject{}, fmt.Errorf("external object upload is not implemented yet; use local storage or add an S3 implementation")
}

func (store *ExternalStore) Delete(ctx context.Context, key string) error {
	_ = ctx
	_ = key
	return nil
}

func (store *ExternalStore) PublicURL(key string) string {
	return DirectURL(store.publicBaseURL, key)
}
