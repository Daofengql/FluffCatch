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
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	CameraMake   string `json:"cameraMake,omitempty"`
	CameraModel  string `json:"cameraModel,omitempty"`
	LensMake     string `json:"lensMake,omitempty"`
	LensModel    string `json:"lensModel,omitempty"`
	TakenAt      string `json:"takenAt,omitempty"`
	ExposureTime string `json:"exposureTime,omitempty"`
	FNumber      string `json:"fNumber,omitempty"`
	ISO          int    `json:"iso,omitempty"`
	ExposureBias string `json:"exposureBias,omitempty"`
	FocalLength  string `json:"focalLength,omitempty"`
}

func ExtractMetadataBytes(content []byte) Metadata {
	metadata := extractEXIF(content)
	if isEmptyMetadata(metadata) {
		return Metadata{}
	}
	size := Metadata{}
	if cfg, _, err := stdimage.DecodeConfig(bytes.NewReader(content)); err == nil {
		size.Width = cfg.Width
		size.Height = cfg.Height
	}
	return mergeMetadata(metadata, size)
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
	if base.LensMake == "" {
		base.LensMake = extra.LensMake
	}
	if base.LensModel == "" {
		base.LensModel = extra.LensModel
	}
	if base.TakenAt == "" {
		base.TakenAt = extra.TakenAt
	}
	if base.ExposureTime == "" {
		base.ExposureTime = extra.ExposureTime
	}
	if base.FNumber == "" {
		base.FNumber = extra.FNumber
	}
	if base.ISO == 0 {
		base.ISO = extra.ISO
	}
	if base.ExposureBias == "" {
		base.ExposureBias = extra.ExposureBias
	}
	if base.FocalLength == "" {
		base.FocalLength = extra.FocalLength
	}
	return base
}

func extractEXIF(content []byte) Metadata {
	if metadata := extractJPEGEXIF(content); !isEmptyMetadata(metadata) {
		return metadata
	}
	if metadata := extractPNGEXIF(content); !isEmptyMetadata(metadata) {
		return metadata
	}
	if metadata := extractWebPEXIF(content); !isEmptyMetadata(metadata) {
		return metadata
	}
	if metadata := extractISOEXIF(content); !isEmptyMetadata(metadata) {
		return metadata
	}
	return extractTIFFEXIF(content)
}

func isEmptyMetadata(metadata Metadata) bool {
	return metadata.Width == 0 &&
		metadata.Height == 0 &&
		metadata.CameraMake == "" &&
		metadata.CameraModel == "" &&
		metadata.LensMake == "" &&
		metadata.LensModel == "" &&
		metadata.TakenAt == "" &&
		metadata.ExposureTime == "" &&
		metadata.FNumber == "" &&
		metadata.ISO == 0 &&
		metadata.ExposureBias == "" &&
		metadata.FocalLength == ""
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

func extractPNGEXIF(content []byte) Metadata {
	if len(content) < 8 || !bytes.Equal(content[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return Metadata{}
	}
	for offset := 8; offset+12 <= len(content); {
		chunkLen := int(binary.BigEndian.Uint32(content[offset : offset+4]))
		chunkType := content[offset+4 : offset+8]
		dataStart := offset + 8
		dataEnd := dataStart + chunkLen
		if chunkLen < 0 || dataEnd+4 > len(content) {
			return Metadata{}
		}
		if bytes.Equal(chunkType, []byte("eXIf")) {
			return parseTIFFMetadata(content[dataStart:dataEnd])
		}
		offset = dataEnd + 4
	}
	return Metadata{}
}

func extractWebPEXIF(content []byte) Metadata {
	if len(content) < 12 || !bytes.Equal(content[:4], []byte("RIFF")) || !bytes.Equal(content[8:12], []byte("WEBP")) {
		return Metadata{}
	}
	for offset := 12; offset+8 <= len(content); {
		chunkType := content[offset : offset+4]
		chunkLen := int(binary.LittleEndian.Uint32(content[offset+4 : offset+8]))
		dataStart := offset + 8
		dataEnd := dataStart + chunkLen
		if chunkLen < 0 || dataEnd > len(content) {
			return Metadata{}
		}
		if bytes.Equal(chunkType, []byte("EXIF")) {
			data := content[dataStart:dataEnd]
			data = bytes.TrimPrefix(data, []byte("Exif\x00\x00"))
			return parseTIFFMetadata(data)
		}
		offset = dataEnd
		if chunkLen%2 == 1 {
			offset++
		}
	}
	return Metadata{}
}

func extractTIFFEXIF(content []byte) Metadata {
	if len(content) < 8 {
		return Metadata{}
	}
	if bytes.Equal(content[:2], []byte("II")) || bytes.Equal(content[:2], []byte("MM")) {
		return parseTIFFMetadata(content)
	}
	return Metadata{}
}

func extractISOEXIF(content []byte) Metadata {
	if len(content) < 12 || !isISOImage(content) {
		return Metadata{}
	}
	for _, box := range parseISOBoxes(content, 0, len(content)) {
		if box.typ != "meta" {
			continue
		}
		if metadata := extractMetaBoxEXIF(content, box); !isEmptyMetadata(metadata) {
			return metadata
		}
	}
	return Metadata{}
}

func extractMetaBoxEXIF(content []byte, meta isoBox) Metadata {
	if meta.dataStart+4 > meta.dataEnd {
		return Metadata{}
	}
	children := parseISOBoxes(content, meta.dataStart+4, meta.dataEnd)
	exifIDs := map[uint32]struct{}{}
	for _, child := range children {
		if child.typ == "iinf" {
			for _, id := range parseIINFExifIDs(content[child.dataStart:child.dataEnd]) {
				exifIDs[id] = struct{}{}
			}
		}
	}
	if len(exifIDs) == 0 {
		return Metadata{}
	}
	for _, child := range children {
		if child.typ != "iloc" {
			continue
		}
		for _, payload := range parseILOCItemPayloads(content, content[child.dataStart:child.dataEnd], exifIDs) {
			if metadata := parseTIFFMetadata(exifPayloadToTIFF(payload)); !isEmptyMetadata(metadata) {
				return metadata
			}
		}
	}
	return Metadata{}
}

type isoBox struct {
	typ       string
	start     int
	end       int
	dataStart int
	dataEnd   int
}

func parseISOBoxes(content []byte, start int, end int) []isoBox {
	if start < 0 || end > len(content) || start >= end {
		return nil
	}
	boxes := []isoBox{}
	for offset := start; offset+8 <= end; {
		size := uint64(binary.BigEndian.Uint32(content[offset : offset+4]))
		headerSize := 8
		if size == 1 {
			if offset+16 > end {
				break
			}
			size = binary.BigEndian.Uint64(content[offset+8 : offset+16])
			headerSize = 16
		} else if size == 0 {
			size = uint64(end - offset)
		}
		if size < uint64(headerSize) || uint64(offset)+size > uint64(end) {
			break
		}
		boxEnd := offset + int(size)
		boxes = append(boxes, isoBox{
			typ:       string(content[offset+4 : offset+8]),
			start:     offset,
			end:       boxEnd,
			dataStart: offset + headerSize,
			dataEnd:   boxEnd,
		})
		offset = boxEnd
	}
	return boxes
}

func isISOImage(content []byte) bool {
	if len(content) < 12 || string(content[4:8]) != "ftyp" {
		return false
	}
	size := int(binary.BigEndian.Uint32(content[0:4]))
	if size < 16 || size > len(content) {
		return false
	}
	for _, brand := range isoBrands(content[8:size]) {
		if isSupportedISOImageBrand(brand) {
			return true
		}
	}
	return false
}

func isoBrands(data []byte) []string {
	if len(data) < 4 {
		return nil
	}
	brands := []string{string(data[:4])}
	for offset := 8; offset+4 <= len(data); offset += 4 {
		brands = append(brands, string(data[offset:offset+4]))
	}
	return brands
}

func isSupportedISOImageBrand(brand string) bool {
	switch brand {
	case "heic", "heix", "hevc", "hevx", "heim", "heis", "hevm", "hevs", "heif", "mif1", "msf1", "avif", "avis":
		return true
	default:
		return false
	}
}

func parseIINFExifIDs(data []byte) []uint32 {
	if len(data) < 6 {
		return nil
	}
	version := data[0]
	offset := 4
	entryCount := 0
	if version == 0 {
		if offset+2 > len(data) {
			return nil
		}
		entryCount = int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
	} else {
		if offset+4 > len(data) {
			return nil
		}
		entryCount = int(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4
	}
	ids := []uint32{}
	for i := 0; i < entryCount && offset+8 <= len(data); i++ {
		size := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		if size < 8 || offset+size > len(data) {
			break
		}
		if string(data[offset+4:offset+8]) == "infe" {
			if id, ok := parseINFEExifID(data[offset+8 : offset+size]); ok {
				ids = append(ids, id)
			}
		}
		offset += size
	}
	return ids
}

func parseINFEExifID(data []byte) (uint32, bool) {
	if len(data) < 4 {
		return 0, false
	}
	version := data[0]
	offset := 4
	switch version {
	case 2:
		if offset+8 > len(data) {
			return 0, false
		}
		id := uint32(binary.BigEndian.Uint16(data[offset : offset+2]))
		itemType := string(data[offset+4 : offset+8])
		return id, itemType == "Exif"
	case 3:
		if offset+10 > len(data) {
			return 0, false
		}
		id := binary.BigEndian.Uint32(data[offset : offset+4])
		itemType := string(data[offset+6 : offset+10])
		return id, itemType == "Exif"
	default:
		if bytes.Contains(data, []byte("Exif")) {
			return 1, true
		}
		return 0, false
	}
}

func parseILOCItemPayloads(content []byte, data []byte, wantedIDs map[uint32]struct{}) [][]byte {
	if len(data) < 8 {
		return nil
	}
	version := data[0]
	offsetSize := int(data[4] >> 4)
	lengthSize := int(data[4] & 0x0f)
	baseOffsetSize := int(data[5] >> 4)
	indexSize := 0
	if version == 1 || version == 2 {
		indexSize = int(data[5] & 0x0f)
	}
	offset := 6
	itemCount := 0
	if version < 2 {
		if offset+2 > len(data) {
			return nil
		}
		itemCount = int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
	} else {
		if offset+4 > len(data) {
			return nil
		}
		itemCount = int(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4
	}
	payloads := [][]byte{}
	for i := 0; i < itemCount; i++ {
		itemID, ok := readILOCInt(data, &offset, map[bool]int{true: 4, false: 2}[version == 2])
		if !ok {
			return payloads
		}
		constructionMethod := uint64(0)
		if version == 1 || version == 2 {
			if offset+2 > len(data) {
				return payloads
			}
			constructionMethod = uint64(binary.BigEndian.Uint16(data[offset:offset+2]) & 0x000f)
			offset += 2
		}
		if _, ok := readILOCInt(data, &offset, 2); !ok {
			return payloads
		}
		baseOffset, ok := readILOCInt(data, &offset, baseOffsetSize)
		if !ok {
			return payloads
		}
		extentCount, ok := readILOCInt(data, &offset, 2)
		if !ok {
			return payloads
		}
		_, wanted := wantedIDs[uint32(itemID)]
		for extent := uint64(0); extent < extentCount; extent++ {
			if indexSize > 0 {
				if _, ok := readILOCInt(data, &offset, indexSize); !ok {
					return payloads
				}
			}
			extentOffset, ok := readILOCInt(data, &offset, offsetSize)
			if !ok {
				return payloads
			}
			extentLength, ok := readILOCInt(data, &offset, lengthSize)
			if !ok {
				return payloads
			}
			if !wanted || constructionMethod != 0 || extentLength == 0 {
				continue
			}
			start := baseOffset + extentOffset
			end := start + extentLength
			if end > uint64(len(content)) || start > end {
				continue
			}
			payloads = append(payloads, content[start:end])
		}
	}
	return payloads
}

func readILOCInt(data []byte, offset *int, size int) (uint64, bool) {
	if size == 0 {
		return 0, true
	}
	if size < 0 || size > 8 || *offset+size > len(data) {
		return 0, false
	}
	var value uint64
	for i := 0; i < size; i++ {
		value = value<<8 | uint64(data[*offset+i])
	}
	*offset += size
	return value, true
}

func exifPayloadToTIFF(data []byte) []byte {
	data = bytes.TrimPrefix(data, []byte("Exif\x00\x00"))
	for offset := 0; offset+8 <= len(data) && offset <= 16; offset++ {
		if (bytes.Equal(data[offset:offset+2], []byte("II")) || bytes.Equal(data[offset:offset+2], []byte("MM"))) &&
			(binary.BigEndian.Uint16(data[offset+2:offset+4]) == 42 || binary.LittleEndian.Uint16(data[offset+2:offset+4]) == 42) {
			return data[offset:]
		}
	}
	return data
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
		if exifMetadata.TakenAt != "" {
			metadata.TakenAt = ""
		}
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
		case 0x9003:
			if parsed := parseEXIFTime(cleanEXIFString(value)); parsed != "" {
				metadata.TakenAt = parsed
			}
		case 0x0132:
			if metadata.TakenAt == "" {
				if parsed := parseEXIFTime(cleanEXIFString(value)); parsed != "" {
					metadata.TakenAt = parsed
				}
			}
		case 0xa433:
			metadata.LensMake = cleanEXIFString(value)
		case 0xa434:
			metadata.LensModel = cleanEXIFString(value)
		case 0xa002:
			metadata.Width = int(exifUnsigned(value, order, fieldType))
		case 0xa003:
			metadata.Height = int(exifUnsigned(value, order, fieldType))
		case 0x829a:
			metadata.ExposureTime = formatExposureTime(exifRational(value, order, fieldType))
		case 0x829d:
			metadata.FNumber = formatFNumber(exifRational(value, order, fieldType))
		case 0x8827:
			metadata.ISO = int(exifUnsigned(value, order, fieldType))
		case 0x9204:
			metadata.ExposureBias = formatExposureBias(exifRational(value, order, fieldType))
		case 0x920a:
			metadata.FocalLength = formatFocalLength(exifRational(value, order, fieldType))
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

func exifUnsigned(value []byte, order binary.ByteOrder, fieldType uint16) uint32 {
	switch fieldType {
	case 3:
		if len(value) >= 2 {
			return uint32(order.Uint16(value[:2]))
		}
	case 4:
		if len(value) >= 4 {
			return order.Uint32(value[:4])
		}
	}
	return 0
}

func exifRational(value []byte, order binary.ByteOrder, fieldType uint16) float64 {
	if len(value) < 8 {
		return 0
	}
	switch fieldType {
	case 5:
		numerator := order.Uint32(value[:4])
		denominator := order.Uint32(value[4:8])
		if denominator == 0 {
			return 0
		}
		return float64(numerator) / float64(denominator)
	case 10:
		numerator := int32(order.Uint32(value[:4]))
		denominator := int32(order.Uint32(value[4:8]))
		if denominator == 0 {
			return 0
		}
		return float64(numerator) / float64(denominator)
	default:
		return 0
	}
}

func formatExposureTime(value float64) string {
	if value <= 0 {
		return ""
	}
	if value < 1 {
		denominator := int(math.Round(1 / value))
		if denominator > 0 {
			return fmt.Sprintf("1/%d s", denominator)
		}
	}
	return fmt.Sprintf("%.3g s", value)
}

func formatFNumber(value float64) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("f/%.1f", value)
}

func formatExposureBias(value float64) string {
	if value == 0 {
		return "0 EV"
	}
	return fmt.Sprintf("%+.1f EV", value)
}

func formatFocalLength(value float64) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%.1f mm", value)
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
