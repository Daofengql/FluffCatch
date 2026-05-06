package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Store struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
}

func NewS3Store(cfg S3Config, publicBaseURL string) (*S3Store, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("accessKey and secretKey are required")
	}

	endpoint := cfg.Endpoint
	if endpoint != "" {
		endpoint = strings.TrimRight(endpoint, "/")
	}

	client := s3.New(s3.Options{
		Credentials: awscreds.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		Region:      cfg.Region,
		BaseEndpoint: func() *string {
			if endpoint != "" {
				return aws.String(endpoint)
			}
			return nil
		}(),
		UsePathStyle: endpoint != "",
	})

	return &S3Store{
		client:        client,
		bucket:        cfg.Bucket,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}, nil
}

func (store *S3Store) Put(ctx context.Context, object Object) (StoredObject, error) {
	_, err := store.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(store.bucket),
		Key:         aws.String(object.Key),
		Body:        object.Content,
		ContentType: aws.String(object.ContentType),
	})
	if err != nil {
		return StoredObject{}, fmt.Errorf("s3 put %s: %w", object.Key, err)
	}

	return StoredObject{
		Key: object.Key,
		URL: store.PublicURL(object.Key),
	}, nil
}

func (store *S3Store) Delete(ctx context.Context, key string) error {
	_, err := store.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isS3NotFoundError(err) {
			return nil
		}
		return fmt.Errorf("s3 delete %s: %w", key, err)
	}
	return nil
}

func (store *S3Store) PublicURL(key string) string {
	return DirectURL(store.publicBaseURL, key)
}

func isS3NotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "NoSuchKey") ||
		strings.Contains(msg, "NotFound") ||
		strings.Contains(msg, "StatusCode: 404")
}
