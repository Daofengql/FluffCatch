package image

import (
	"encoding/binary"
	"testing"
)

func TestExtractMetadataBytesPrefersDateTimeOriginal(t *testing.T) {
	content := jpegWithEXIF(t, "2026:05:01 10:11:12", "2026:05:02 13:14:15")

	metadata := ExtractMetadataBytes(content)

	if metadata.TakenAt == "" {
		t.Fatal("expected takenAt from EXIF")
	}
	if metadata.TakenAt[:19] != "2026-05-02T13:14:15" {
		t.Fatalf("expected DateTimeOriginal, got %q", metadata.TakenAt)
	}
}

func TestExtractMetadataBytesReadsCommonEXIFFields(t *testing.T) {
	metadata := ExtractMetadataBytes(jpegWithEXIF(t, "2026:05:01 10:11:12", "2026:05:02 13:14:15"))

	if metadata.CameraMake != "Canon" {
		t.Fatalf("expected camera make, got %q", metadata.CameraMake)
	}
	if metadata.CameraModel != "EOS R5" {
		t.Fatalf("expected camera model, got %q", metadata.CameraModel)
	}
	if metadata.LensModel != "RF 50mm F1.2L" {
		t.Fatalf("expected lens model, got %q", metadata.LensModel)
	}
	if metadata.ExposureTime != "1/125 s" {
		t.Fatalf("expected exposure time, got %q", metadata.ExposureTime)
	}
	if metadata.FNumber != "f/2.8" {
		t.Fatalf("expected f-number, got %q", metadata.FNumber)
	}
	if metadata.ISO != 800 {
		t.Fatalf("expected ISO 800, got %d", metadata.ISO)
	}
	if metadata.ExposureBias != "+0.3 EV" {
		t.Fatalf("expected exposure bias, got %q", metadata.ExposureBias)
	}
	if metadata.FocalLength != "50.0 mm" {
		t.Fatalf("expected focal length, got %q", metadata.FocalLength)
	}
}

func TestExtractMetadataBytesReadsEXIFFromCommonContainers(t *testing.T) {
	tests := map[string][]byte{
		"jpeg": jpegWithEXIF(t, "2026:05:01 10:11:12", "2026:05:02 13:14:15"),
		"png":  pngWithEXIF(t, testTIFFWithEXIF(t, "2026:05:01 10:11:12", "2026:05:02 13:14:15")),
		"webp": webpWithEXIF(t, testTIFFWithEXIF(t, "2026:05:01 10:11:12", "2026:05:02 13:14:15")),
		"heic": isoImageWithEXIF(t, "heic", testTIFFWithEXIF(t, "2026:05:01 10:11:12", "2026:05:02 13:14:15")),
		"heif": isoImageWithEXIF(t, "heif", testTIFFWithEXIF(t, "2026:05:01 10:11:12", "2026:05:02 13:14:15")),
		"avif": isoImageWithEXIF(t, "avif", testTIFFWithEXIF(t, "2026:05:01 10:11:12", "2026:05:02 13:14:15")),
		"tiff": testTIFFWithEXIF(t, "2026:05:01 10:11:12", "2026:05:02 13:14:15"),
	}

	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			metadata := ExtractMetadataBytes(content)
			if metadata.TakenAt == "" {
				t.Fatal("expected takenAt from EXIF")
			}
			if metadata.TakenAt[:19] != "2026-05-02T13:14:15" {
				t.Fatalf("expected DateTimeOriginal, got %q", metadata.TakenAt)
			}
			if metadata.CameraModel != "EOS R5" {
				t.Fatalf("expected camera model, got %q", metadata.CameraModel)
			}
		})
	}
}

func TestExtractMetadataBytesIgnoresImagesWithoutEXIF(t *testing.T) {
	content := []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0, 0, 0, 13, 'I', 'H', 'D', 'R',
		0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0,
		0, 0, 0, 0,
	}

	metadata := ExtractMetadataBytes(content)

	if !isEmptyMetadata(metadata) {
		t.Fatalf("expected empty metadata without EXIF, got %#v", metadata)
	}
}

func jpegWithEXIF(t *testing.T, ifdDateTime string, originalDateTime string) []byte {
	t.Helper()

	tiff := testTIFFWithEXIF(t, ifdDateTime, originalDateTime)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segmentLen := len(payload) + 2
	if segmentLen > 0xffff {
		t.Fatal("test EXIF payload too large")
	}

	content := []byte{0xff, 0xd8, 0xff, 0xe1}
	content = binary.BigEndian.AppendUint16(content, uint16(segmentLen))
	content = append(content, payload...)
	content = append(content, 0xff, 0xd9)
	return content
}

func pngWithEXIF(t *testing.T, tiff []byte) []byte {
	t.Helper()

	content := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	content = binary.BigEndian.AppendUint32(content, uint32(len(tiff)))
	content = append(content, 'e', 'X', 'I', 'f')
	content = append(content, tiff...)
	content = append(content, 0, 0, 0, 0)
	return content
}

func webpWithEXIF(t *testing.T, tiff []byte) []byte {
	t.Helper()

	chunk := append([]byte("EXIF"), binary.LittleEndian.AppendUint32(nil, uint32(len(tiff)))...)
	chunk = append(chunk, tiff...)
	if len(tiff)%2 == 1 {
		chunk = append(chunk, 0)
	}
	content := []byte("RIFF")
	content = binary.LittleEndian.AppendUint32(content, uint32(4+len(chunk)))
	content = append(content, []byte("WEBP")...)
	content = append(content, chunk...)
	return content
}

func isoImageWithEXIF(t *testing.T, brand string, tiff []byte) []byte {
	t.Helper()

	exifPayload := append([]byte{0, 0, 0, 0}, tiff...)
	compatibleBrands := []byte("mif1")
	compatibleBrands = append(compatibleBrands, []byte(brand)...)
	ftyp := isoBoxBytes("ftyp", append(append([]byte(brand), 0, 0, 0, 0), compatibleBrands...))
	infe := isoFullBoxBytes("infe", 2, 0, append(append(append(append([]byte{}, 0, 1), 0, 0), []byte("Exif")...), []byte("Exif\x00")...))
	iinf := isoFullBoxBytes("iinf", 0, 0, append(append([]byte{}, 0, 1), infe...))

	placeholderILOC := isoFullBoxBytes("iloc", 0, 0, []byte{
		0x44, 0x40,
		0, 1,
		0, 1,
		0, 0,
		0, 0, 0, 0,
		0, 1,
		0, 0, 0, 0,
		0, 0, 0, 0,
	})
	meta := isoFullBoxBytes("meta", 0, 0, append(iinf, placeholderILOC...))
	mdatOffset := len(ftyp) + len(meta) + 8
	ilocPayload := []byte{
		0x44, 0x40,
		0, 1,
		0, 1,
		0, 0,
	}
	ilocPayload = binary.BigEndian.AppendUint32(ilocPayload, uint32(mdatOffset))
	ilocPayload = append(ilocPayload, 0, 1)
	ilocPayload = binary.BigEndian.AppendUint32(ilocPayload, 0)
	ilocPayload = binary.BigEndian.AppendUint32(ilocPayload, uint32(len(exifPayload)))
	iloc := isoFullBoxBytes("iloc", 0, 0, ilocPayload)
	meta = isoFullBoxBytes("meta", 0, 0, append(iinf, iloc...))
	mdat := isoBoxBytes("mdat", exifPayload)

	return append(append(ftyp, meta...), mdat...)
}

func testTIFFWithEXIF(t *testing.T, ifdDateTime string, originalDateTime string) []byte {
	t.Helper()

	tiff := make([]byte, 0, 512)
	tiff = append(tiff, 'I', 'I')
	tiff = binary.LittleEndian.AppendUint16(tiff, 42)
	tiff = binary.LittleEndian.AppendUint32(tiff, 8)

	const ifd0Offset = 8
	const ifd0Entries = 5
	if len(tiff) != ifd0Offset {
		t.Fatalf("unexpected tiff offset: %d", len(tiff))
	}
	tiff = binary.LittleEndian.AppendUint16(tiff, ifd0Entries)
	ifd0EntryStart := len(tiff)
	tiff = append(tiff, make([]byte, ifd0Entries*12)...)
	tiff = binary.LittleEndian.AppendUint32(tiff, 0)

	makeOffset := len(tiff)
	tiff = append(tiff, []byte("Canon\x00")...)
	modelOffset := len(tiff)
	tiff = append(tiff, []byte("EOS R5\x00")...)
	ifdDateOffset := len(tiff)
	tiff = append(tiff, []byte(ifdDateTime+"\x00")...)

	exifIFDOffset := len(tiff)
	const exifEntries = 10
	tiff = binary.LittleEndian.AppendUint16(tiff, exifEntries)
	exifEntryStart := len(tiff)
	tiff = append(tiff, make([]byte, exifEntries*12)...)
	tiff = binary.LittleEndian.AppendUint32(tiff, 0)
	originalDateOffset := len(tiff)
	tiff = append(tiff, []byte(originalDateTime+"\x00")...)
	lensMakeOffset := len(tiff)
	tiff = append(tiff, []byte("Canon\x00")...)
	lensModelOffset := len(tiff)
	tiff = append(tiff, []byte("RF 50mm F1.2L\x00")...)
	exposureOffset := appendRational(&tiff, 1, 125)
	fNumberOffset := appendRational(&tiff, 28, 10)
	exposureBiasOffset := appendSignedRational(&tiff, 1, 3)
	focalLengthOffset := appendRational(&tiff, 50, 1)

	writeIFDEntry(tiff[ifd0EntryStart:ifd0EntryStart+12], 0x010f, 2, 6, uint32(makeOffset))
	writeIFDEntry(tiff[ifd0EntryStart+12:ifd0EntryStart+24], 0x0110, 2, 7, uint32(modelOffset))
	writeIFDEntry(tiff[ifd0EntryStart+24:ifd0EntryStart+36], 0x0132, 2, uint32(len(ifdDateTime)+1), uint32(ifdDateOffset))
	writeIFDEntry(tiff[ifd0EntryStart+36:ifd0EntryStart+48], 0x8769, 4, 1, uint32(exifIFDOffset))
	writeIFDEntry(tiff[ifd0EntryStart+48:ifd0EntryStart+60], 0x0100, 4, 1, 4000)

	writeIFDEntry(tiff[exifEntryStart:exifEntryStart+12], 0x9003, 2, uint32(len(originalDateTime)+1), uint32(originalDateOffset))
	writeIFDEntry(tiff[exifEntryStart+12:exifEntryStart+24], 0xa433, 2, 6, uint32(lensMakeOffset))
	writeIFDEntry(tiff[exifEntryStart+24:exifEntryStart+36], 0xa434, 2, 14, uint32(lensModelOffset))
	writeIFDEntry(tiff[exifEntryStart+36:exifEntryStart+48], 0x829a, 5, 1, uint32(exposureOffset))
	writeIFDEntry(tiff[exifEntryStart+48:exifEntryStart+60], 0x829d, 5, 1, uint32(fNumberOffset))
	writeIFDEntry(tiff[exifEntryStart+60:exifEntryStart+72], 0x8827, 3, 1, 800)
	writeIFDEntry(tiff[exifEntryStart+72:exifEntryStart+84], 0x9204, 10, 1, uint32(exposureBiasOffset))
	writeIFDEntry(tiff[exifEntryStart+84:exifEntryStart+96], 0x920a, 5, 1, uint32(focalLengthOffset))
	writeIFDEntry(tiff[exifEntryStart+96:exifEntryStart+108], 0xa002, 4, 1, 4000)
	writeIFDEntry(tiff[exifEntryStart+108:exifEntryStart+120], 0xa003, 4, 1, 3000)

	return tiff
}

func appendRational(target *[]byte, numerator uint32, denominator uint32) int {
	offset := len(*target)
	*target = binary.LittleEndian.AppendUint32(*target, numerator)
	*target = binary.LittleEndian.AppendUint32(*target, denominator)
	return offset
}

func appendSignedRational(target *[]byte, numerator int32, denominator int32) int {
	offset := len(*target)
	*target = binary.LittleEndian.AppendUint32(*target, uint32(numerator))
	*target = binary.LittleEndian.AppendUint32(*target, uint32(denominator))
	return offset
}

func isoBoxBytes(boxType string, payload []byte) []byte {
	box := binary.BigEndian.AppendUint32(nil, uint32(len(payload)+8))
	box = append(box, []byte(boxType)...)
	box = append(box, payload...)
	return box
}

func isoFullBoxBytes(boxType string, version byte, flags uint32, payload []byte) []byte {
	fullPayload := []byte{version, byte(flags >> 16), byte(flags >> 8), byte(flags)}
	fullPayload = append(fullPayload, payload...)
	return isoBoxBytes(boxType, fullPayload)
}

func writeIFDEntry(entry []byte, tag uint16, fieldType uint16, count uint32, value uint32) {
	binary.LittleEndian.PutUint16(entry[0:2], tag)
	binary.LittleEndian.PutUint16(entry[2:4], fieldType)
	binary.LittleEndian.PutUint32(entry[4:8], count)
	binary.LittleEndian.PutUint32(entry[8:12], value)
}
