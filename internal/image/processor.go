package image

import (
	"context"
	"io"
)

type Metadata struct {
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	CameraMake  string `json:"cameraMake,omitempty"`
	CameraModel string `json:"cameraModel,omitempty"`
	TakenAt     string `json:"takenAt,omitempty"`
}

type Thumbnail struct {
	ObjectKey string `json:"objectKey"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

type Processor interface {
	ExtractMetadata(ctx context.Context, content io.Reader) (Metadata, error)
	GenerateThumbnail(ctx context.Context, content io.Reader, objectKey string) (Thumbnail, error)
}

type NoopProcessor struct{}

func (processor NoopProcessor) ExtractMetadata(ctx context.Context, content io.Reader) (Metadata, error) {
	_ = ctx
	_ = content
	return Metadata{}, nil
}

func (processor NoopProcessor) GenerateThumbnail(ctx context.Context, content io.Reader, objectKey string) (Thumbnail, error) {
	_ = ctx
	_ = content
	return Thumbnail{
		ObjectKey: objectKey,
		Width:     0,
		Height:    0,
	}, nil
}
