package image

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	stdimage "image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"math"
	"strings"
	"time"

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

func ExtractMetadataBytes(content []byte) Metadata {
	metadata := Metadata{}
	if cfg, _, err := stdimage.DecodeConfig(bytes.NewReader(content)); err == nil {
		metadata.Width = cfg.Width
		metadata.Height = cfg.Height
	}
	metadata = mergeMetadata(metadata, extractJPEGEXIF(content))
	return metadata
}

func mergeMetadata(base Metadata, extra Metadata) Metadata {
	if base.Width == 0 {
		base.Width = extra.Width
	}
	if base.Height == 0 {
		base.Height = extra.Height
	}
	if base.CameraMake == "" {
		base.CameraMake = extra.CameraMake
	}
	if base.CameraModel == "" {
		base.CameraModel = extra.CameraModel
	}
	if base.TakenAt == "" {
		base.TakenAt = extra.TakenAt
	}
	return base
}

func extractJPEGEXIF(content []byte) Metadata {
	if len(content) < 4 || content[0] != 0xff || content[1] != 0xd8 {
		return Metadata{}
	}
	for offset := 2; offset+4 <= len(content); {
		if content[offset] != 0xff {
			return Metadata{}
		}
		marker := content[offset+1]
		offset += 2
		if marker == 0xda || marker == 0xd9 {
			return Metadata{}
		}
		if offset+2 > len(content) {
			return Metadata{}
		}
		segmentLen := int(binary.BigEndian.Uint16(content[offset : offset+2]))
		offset += 2
		if segmentLen < 2 || offset+segmentLen-2 > len(content) {
			return Metadata{}
		}
		segment := content[offset : offset+segmentLen-2]
		if marker == 0xe1 && len(segment) > 6 && bytes.Equal(segment[:6], []byte("Exif\x00\x00")) {
			return parseTIFFMetadata(segment[6:])
		}
		offset += segmentLen - 2
	}
	return Metadata{}
}

func parseTIFFMetadata(data []byte) Metadata {
	if len(data) < 8 {
		return Metadata{}
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return Metadata{}
	}
	if order.Uint16(data[2:4]) != 42 {
		return Metadata{}
	}
	ifdOffset := int(order.Uint32(data[4:8]))
	metadata, exifOffset := parseIFD(data, order, ifdOffset)
	if exifOffset > 0 {
		exifMetadata, _ := parseIFD(data, order, exifOffset)
		metadata = mergeMetadata(metadata, exifMetadata)
	}
	return metadata
}

func parseIFD(data []byte, order binary.ByteOrder, offset int) (Metadata, int) {
	metadata := Metadata{}
	if offset <= 0 || offset+2 > len(data) {
		return metadata, 0
	}
	count := int(order.Uint16(data[offset : offset+2]))
	pos := offset + 2
	exifOffset := 0
	for i := 0; i < count; i++ {
		entry := pos + i*12
		if entry+12 > len(data) {
			break
		}
		tag := order.Uint16(data[entry : entry+2])
		fieldType := order.Uint16(data[entry+2 : entry+4])
		fieldCount := int(order.Uint32(data[entry+4 : entry+8]))
		value := exifFieldBytes(data, order, entry+8, fieldType, fieldCount)
		switch tag {
		case 0x010f:
			metadata.CameraMake = cleanEXIFString(value)
		case 0x0110:
			metadata.CameraModel = cleanEXIFString(value)
		case 0x8769:
			if len(value) >= 4 {
				exifOffset = int(order.Uint32(value[:4]))
			}
		case 0x9003, 0x0132:
			if parsed := parseEXIFTime(cleanEXIFString(value)); parsed != "" {
				metadata.TakenAt = parsed
			}
		}
	}
	return metadata, exifOffset
}

func exifFieldBytes(data []byte, order binary.ByteOrder, valueOffset int, fieldType uint16, count int) []byte {
	unitSize := 1
	switch fieldType {
	case 3:
		unitSize = 2
	case 4, 9:
		unitSize = 4
	case 5, 10:
		unitSize = 8
	}
	size := unitSize * count
	if size <= 0 {
		return nil
	}
	if size <= 4 {
		return data[valueOffset : valueOffset+4]
	}
	target := int(order.Uint32(data[valueOffset : valueOffset+4]))
	if target < 0 || target+size > len(data) {
		return nil
	}
	return data[target : target+size]
}

func cleanEXIFString(value []byte) string {
	text := strings.TrimRight(string(value), "\x00 ")
	return strings.TrimSpace(text)
}

func parseEXIFTime(value string) string {
	if value == "" {
		return ""
	}
	for _, layout := range []string{"2006:01:02 15:04:05", "2006-01-02 15:04:05", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed.Format(time.RFC3339)
		}
	}
	return ""
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
