package image

import (
	"bytes"
	"context"
	"fmt"
	stdimage "image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"math"

	_ "image/gif"
	_ "image/png"
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

const maxDecodeMegapixels = 80

func GenerateThumbnailBytes(content []byte, maxDimension int) ([]byte, string, error) {
	if maxDimension <= 0 {
		maxDimension = 480
	}

	cfg, _, err := stdimage.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return nil, "", fmt.Errorf("decode image config: %w", err)
	}
	mp := float64(cfg.Width) * float64(cfg.Height) / 1_000_000
	if mp > maxDecodeMegapixels {
		return nil, "", fmt.Errorf("image too large (%.0f MP, max %d MP)", mp, maxDecodeMegapixels)
	}

	source, _, err := stdimage.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	bounds := source.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, "", fmt.Errorf("invalid image dimensions")
	}

	scale := math.Min(float64(maxDimension)/float64(width), float64(maxDimension)/float64(height))
	if scale > 1 {
		scale = 1
	}
	targetWidth := max(1, int(math.Round(float64(width)*scale)))
	targetHeight := max(1, int(math.Round(float64(height)*scale)))

	target := stdimage.NewRGBA(stdimage.Rect(0, 0, targetWidth, targetHeight))
	draw.Draw(target, target.Bounds(), &stdimage.Uniform{C: color.White}, stdimage.Point{}, draw.Src)
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			sourceX := bounds.Min.X + int(float64(x)*float64(width)/float64(targetWidth))
			sourceY := bounds.Min.Y + int(float64(y)*float64(height)/float64(targetHeight))
			target.Set(x, y, source.At(sourceX, sourceY))
		}
	}

	var output bytes.Buffer
	if err := jpeg.Encode(&output, target, &jpeg.Options{Quality: 82}); err != nil {
		return nil, "", fmt.Errorf("encode thumbnail: %w", err)
	}

	return output.Bytes(), "image/jpeg", nil
}

func GenerateBlurredPreviewBytes(content []byte, maxDimension int) ([]byte, string, error) {
	if maxDimension <= 0 {
		maxDimension = 360
	}

	cfg, _, err := stdimage.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return nil, "", fmt.Errorf("decode image config: %w", err)
	}
	mp := float64(cfg.Width) * float64(cfg.Height) / 1_000_000
	if mp > maxDecodeMegapixels {
		return nil, "", fmt.Errorf("image too large (%.0f MP, max %d MP)", mp, maxDecodeMegapixels)
	}

	source, _, err := stdimage.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	bounds := source.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, "", fmt.Errorf("invalid image dimensions")
	}

	scale := math.Min(float64(maxDimension)/float64(width), float64(maxDimension)/float64(height))
	if scale > 1 {
		scale = 1
	}
	targetWidth := max(1, int(math.Round(float64(width)*scale)))
	targetHeight := max(1, int(math.Round(float64(height)*scale)))
	tinyWidth := max(1, targetWidth/24)
	tinyHeight := max(1, targetHeight/24)

	tiny := stdimage.NewRGBA(stdimage.Rect(0, 0, tinyWidth, tinyHeight))
	for y := 0; y < tinyHeight; y++ {
		for x := 0; x < tinyWidth; x++ {
			sourceX := bounds.Min.X + int(float64(x)*float64(width)/float64(tinyWidth))
			sourceY := bounds.Min.Y + int(float64(y)*float64(height)/float64(tinyHeight))
			tiny.Set(x, y, source.At(sourceX, sourceY))
		}
	}

	target := stdimage.NewRGBA(stdimage.Rect(0, 0, targetWidth, targetHeight))
	draw.Draw(target, target.Bounds(), &stdimage.Uniform{C: color.White}, stdimage.Point{}, draw.Src)
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			tinyX := min(tinyWidth-1, int(float64(x)*float64(tinyWidth)/float64(targetWidth)))
			tinyY := min(tinyHeight-1, int(float64(y)*float64(tinyHeight)/float64(targetHeight)))
			target.Set(x, y, tiny.At(tinyX, tinyY))
		}
	}

	var output bytes.Buffer
	if err := jpeg.Encode(&output, target, &jpeg.Options{Quality: 45}); err != nil {
		return nil, "", fmt.Errorf("encode blurred preview: %w", err)
	}

	return output.Bytes(), "image/jpeg", nil
}

func GenerateCoverJPEGBytes(content []byte, targetWidth int, targetHeight int, quality int) ([]byte, string, error) {
	if targetWidth <= 0 || targetHeight <= 0 {
		return nil, "", fmt.Errorf("invalid target dimensions")
	}
	if quality <= 0 || quality > 100 {
		quality = 82
	}

	source, _, err := stdimage.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	bounds := source.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, "", fmt.Errorf("invalid image dimensions")
	}

	targetRatio := float64(targetWidth) / float64(targetHeight)
	sourceRatio := float64(width) / float64(height)
	cropWidth := width
	cropHeight := height
	if sourceRatio > targetRatio {
		cropWidth = max(1, int(math.Round(float64(height)*targetRatio)))
	} else if sourceRatio < targetRatio {
		cropHeight = max(1, int(math.Round(float64(width)/targetRatio)))
	}
	cropX := bounds.Min.X + (width-cropWidth)/2
	cropY := bounds.Min.Y + (height-cropHeight)/2

	target := stdimage.NewRGBA(stdimage.Rect(0, 0, targetWidth, targetHeight))
	draw.Draw(target, target.Bounds(), &stdimage.Uniform{C: color.White}, stdimage.Point{}, draw.Src)
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			sourceX := cropX + min(cropWidth-1, int(float64(x)*float64(cropWidth)/float64(targetWidth)))
			sourceY := cropY + min(cropHeight-1, int(float64(y)*float64(cropHeight)/float64(targetHeight)))
			target.Set(x, y, colorOverWhite(source.At(sourceX, sourceY)))
		}
	}

	var output bytes.Buffer
	if err := jpeg.Encode(&output, target, &jpeg.Options{Quality: quality}); err != nil {
		return nil, "", fmt.Errorf("encode image: %w", err)
	}

	return output.Bytes(), "image/jpeg", nil
}

func colorOverWhite(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	if a == 0xffff {
		return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
	}
	if a == 0 {
		return color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
	return color.RGBA{
		R: uint8((r + 0xffff - a) >> 8),
		G: uint8((g + 0xffff - a) >> 8),
		B: uint8((b + 0xffff - a) >> 8),
		A: 0xff,
	}
}
