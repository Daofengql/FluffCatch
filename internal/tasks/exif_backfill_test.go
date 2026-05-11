package tasks

import (
	"encoding/json"
	"testing"
	"time"

	appimage "fluffcatch/internal/image"
)

func TestIsEmptyImageMetadata(t *testing.T) {
	if !isEmptyImageMetadata(appimage.Metadata{}) {
		t.Fatal("expected empty metadata to be empty")
	}
	if isEmptyImageMetadata(appimage.Metadata{CameraModel: "EOS R5"}) {
		t.Fatal("expected camera metadata to be non-empty")
	}
}

func TestBackfillMetadataEncodingOverwritesExistingShape(t *testing.T) {
	metadata := appimage.Metadata{
		CameraMake:   "Canon",
		CameraModel:  "EOS R5",
		TakenAt:      "2026-05-02T13:14:15+08:00",
		ExposureTime: "1/125 s",
		FNumber:      "f/2.8",
		ISO:          800,
	}

	raw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if _, ok := decoded["legacyField"]; ok {
		t.Fatal("unexpected legacy field in full overwrite metadata")
	}
	if decoded["cameraModel"] != "EOS R5" {
		t.Fatalf("expected camera model, got %#v", decoded["cameraModel"])
	}
}

func TestEqualTimePtr(t *testing.T) {
	now := time.Date(2026, 5, 2, 13, 14, 15, 0, time.UTC)
	same := now
	other := now.Add(time.Second)

	if !equalTimePtr(&now, &same) {
		t.Fatal("expected same times to match")
	}
	if equalTimePtr(&now, &other) {
		t.Fatal("expected different times not to match")
	}
	if equalTimePtr(&now, nil) {
		t.Fatal("expected nil and non-nil not to match")
	}
}
