package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	oss "github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type OSSStore struct {
	bucket        *oss.Bucket
	publicBaseURL string
}

func NewOSSStore(cfg S3Config, publicBaseURL string) (*OSSStore, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("oss endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("oss bucket is required")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("oss accessKey and secretKey are required")
	}

	client, err := oss.New(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("create oss client: %w", err)
	}

	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("get oss bucket %s: %w", cfg.Bucket, err)
	}

	return &OSSStore{
		bucket:        bucket,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}, nil
}

func (store *OSSStore) Put(ctx context.Context, object Object) (StoredObject, error) {
	data, err := io.ReadAll(object.Content)
	if err != nil {
		return StoredObject{}, fmt.Errorf("oss read content: %w", err)
	}

	err = store.bucket.PutObject(object.Key, bytes.NewReader(data), oss.ContentType(object.ContentType))
	if err != nil {
		return StoredObject{}, fmt.Errorf("oss put %s: %w", object.Key, err)
	}

	return StoredObject{
		Key: object.Key,
		URL: store.PublicURL(object.Key),
	}, nil
}

func (store *OSSStore) Delete(_ context.Context, key string) error {
	err := store.bucket.DeleteObject(key)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") {
			return nil
		}
		return fmt.Errorf("oss delete %s: %w", key, err)
	}
	return nil
}

func (store *OSSStore) PublicURL(key string) string {
	return DirectURL(store.publicBaseURL, key)
}
