package storage

import (
	"context"
	"fmt"
	"strconv"
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
	err := store.bucket.PutObject(object.Key, object.Content, oss.ContentType(object.ContentType))
	if err != nil {
		return StoredObject{}, fmt.Errorf("oss put %s: %w", object.Key, err)
	}

	return StoredObject{
		Key: object.Key,
		URL: store.PublicURL(object.Key),
	}, nil
}

func (store *OSSStore) Get(_ context.Context, key string) (ObjectReader, error) {
	body, err := store.bucket.GetObject(key)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "404") {
			return ObjectReader{}, fmt.Errorf("object not found")
		}
		return ObjectReader{}, fmt.Errorf("oss get %s: %w", key, err)
	}

	meta, err := store.bucket.GetObjectDetailedMeta(key)
	if err != nil {
		return ObjectReader{Content: body, ContentLength: -1}, nil
	}

	contentLength := int64(-1)
	if parsed, err := strconv.ParseInt(meta.Get("Content-Length"), 10, 64); err == nil {
		contentLength = parsed
	}

	return ObjectReader{
		Content:       body,
		ContentType:   meta.Get("Content-Type"),
		ContentLength: contentLength,
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
