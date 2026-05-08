package uploads

import "time"

type SubmissionStatus string

const (
	SubmissionPending  SubmissionStatus = "pending"
	SubmissionApproved SubmissionStatus = "approved"
)

type Submission struct {
	ID               int64            `json:"id"`
	EventID          int64            `json:"eventId"`
	StoragePolicyID  string           `json:"storagePolicyId"`
	ObjectKey        string           `json:"objectKey"`
	URL              string           `json:"url"`
	ThumbnailKey     string           `json:"thumbnailKey,omitempty"`
	ThumbnailURL     string           `json:"thumbnailUrl,omitempty"`
	ContentHash      string           `json:"contentHash"`
	ContentType      string           `json:"contentType"`
	SizeBytes        int64            `json:"sizeBytes"`
	PhotographerName string           `json:"photographerName,omitempty"`
	Tags             []string         `json:"tags"`
	Status           SubmissionStatus `json:"status"`
	CreatedAt        time.Time        `json:"createdAt"`
}

type CreateSubmissionRequest struct {
	SubmissionPassword string   `json:"submissionPassword"`
	PhotographerName   string   `json:"photographerName"`
	Tags               []string `json:"tags"`
}

type BatchRequest struct {
	SubmissionIDs []int64 `json:"submissionIds"`
	Visibility    string  `json:"visibility"`
}

type BatchResponse struct {
	Processed int    `json:"processed"`
	Message   string `json:"message"`
}
