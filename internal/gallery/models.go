package gallery

import "time"

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

type Photo struct {
	ID               int64      `json:"id"`
	EventID          int64      `json:"eventId"`
	StoragePolicyID  string     `json:"storagePolicyId"`
	ObjectKey        string     `json:"objectKey"`
	URL              string     `json:"url"`
	ThumbnailKey     string     `json:"thumbnailKey,omitempty"`
	ThumbnailURL     string     `json:"thumbnailUrl,omitempty"`
	AccessGranted    bool       `json:"accessGranted"`
	ContentHash      string     `json:"contentHash"`
	ContentType      string     `json:"contentType"`
	SizeBytes        int64      `json:"sizeBytes"`
	LikeCount        int64      `json:"likeCount"`
	Liked            bool       `json:"liked"`
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

type Page struct {
	Items      []Photo `json:"photos"`
	Total      int64   `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
	TotalPages int     `json:"totalPages"`
}

type ListOptions struct {
	Admin           bool
	PrivateAccess   bool
	FingerprintHash string
	Page            int
	PageSize        int
	Visibility      Visibility
}

type UpdatePhotoRequest struct {
	PhotographerName string     `json:"photographerName"`
	Visibility       Visibility `json:"visibility"`
	Tags             []string   `json:"tags"`
}

type BatchUpdatePhotosRequest struct {
	PhotoIDs   []int64    `json:"photoIds"`
	Visibility Visibility `json:"visibility"`
}

type LikeResult struct {
	PhotoID   int64 `json:"photoId"`
	LikeCount int64 `json:"likeCount"`
	Liked     bool  `json:"liked"`
	JustLiked bool  `json:"justLiked"`
}
