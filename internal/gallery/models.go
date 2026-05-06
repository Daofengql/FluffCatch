package gallery

import "time"

type Visibility string

const (
	VisibilityPublic    Visibility = "public"
	VisibilityProtected Visibility = "protected"
	VisibilityPrivate   Visibility = "private"
)

type Photo struct {
	ID               int64      `json:"id"`
	EventID          int64      `json:"eventId"`
	StoragePolicyID  string     `json:"storagePolicyId"`
	ObjectKey        string     `json:"objectKey"`
	URL              string     `json:"url"`
	ThumbnailKey     string     `json:"thumbnailKey,omitempty"`
	ThumbnailURL     string     `json:"thumbnailUrl,omitempty"`
	OriginalFilename string     `json:"originalFilename"`
	PhotographerName string     `json:"photographerName,omitempty"`
	Visibility       Visibility `json:"visibility"`
	Tags             []Tag      `json:"tags"`
	TakenAt          *time.Time `json:"takenAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type Tag struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type UpdatePhotoRequest struct {
	Visibility     Visibility `json:"visibility"`
	AccessPassword string     `json:"accessPassword"`
	Tags           []string   `json:"tags"`
}
