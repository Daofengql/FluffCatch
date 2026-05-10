package uploads

import (
	"encoding/binary"
	"testing"
)

func TestDetectUploadContentTypeSupportsISOImageBrands(t *testing.T) {
	tests := map[string]string{
		"heic": "image/heic",
		"heif": "image/heif",
		"avif": "image/avif",
	}

	for brand, expected := range tests {
		t.Run(brand, func(t *testing.T) {
			if actual := detectUploadContentType(testFTYP(brand)); actual != expected {
				t.Fatalf("expected %s, got %s", expected, actual)
			}
		})
	}
}

func TestDetectUploadContentTypeDoesNotTrustExtension(t *testing.T) {
	if actual := detectUploadContentType([]byte("not an image")); actual == "image/heic" || actual == "image/heif" || actual == "image/avif" {
		t.Fatalf("unexpected ISO image type for non-image data: %s", actual)
	}
}

func testFTYP(brand string) []byte {
	payload := append(append([]byte(brand), 0, 0, 0, 0), []byte("mif1"+brand)...)
	content := binary.BigEndian.AppendUint32(nil, uint32(len(payload)+8))
	content = append(content, []byte("ftyp")...)
	content = append(content, payload...)
	return content
}
