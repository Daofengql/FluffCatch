package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/tencentyun/cos-go-sdk-v5"
)

type COSStore struct {
	client        *cos.Client
	publicBaseURL string
}

func NewCOSStore(cfg S3Config, publicBaseURL string) (*COSStore, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("cos endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("cos bucket is required")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("cos accessKey and secretKey are required")
	}

	bucketURL, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse cos endpoint: %w", err)
	}

	serviceURL, err := url.Parse(fmt.Sprintf("https://cos.%s.myqcloud.com", bucketURL.Hostname()))
	if err != nil {
		return nil, fmt.Errorf("construct cos service url: %w", err)
	}

	client := cos.NewClient(&cos.BaseURL{
		BucketURL:  bucketURL,
		ServiceURL: serviceURL,
	}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.AccessKey,
			SecretKey: cfg.SecretKey,
		},
	})

	return &COSStore{
		client:        client,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}, nil
}

func (store *COSStore) Put(ctx context.Context, object Object) (StoredObject, error) {
	_, err := store.client.Object.Put(ctx, object.Key, object.Content, &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: object.ContentType,
		},
	})
	if err != nil {
		return StoredObject{}, fmt.Errorf("cos put %s: %w", object.Key, err)
	}

	return StoredObject{
		Key: object.Key,
		URL: store.PublicURL(object.Key),
	}, nil
}

func (store *COSStore) Get(ctx context.Context, key string) (ObjectReader, error) {
	response, err := store.client.Object.Get(ctx, key, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "NoSuchKey") {
			return ObjectReader{}, fmt.Errorf("object not found")
		}
		return ObjectReader{}, fmt.Errorf("cos get %s: %w", key, err)
	}

	return ObjectReader{
		Content:       response.Body,
		ContentType:   response.Header.Get("Content-Type"),
		ContentLength: response.ContentLength,
	}, nil
}

func (store *COSStore) Delete(ctx context.Context, key string) error {
	_, err := store.client.Object.Delete(ctx, key)
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "NoSuchKey") {
			return nil
		}
		return fmt.Errorf("cos delete %s: %w", key, err)
	}
	return nil
}

func (store *COSStore) PublicURL(key string) string {
	return DirectURL(store.publicBaseURL, key)
}
