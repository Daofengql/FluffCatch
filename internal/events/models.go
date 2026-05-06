package events

import "time"

type Event struct {
	ID                 int64      `json:"id"`
	Slug               string     `json:"slug"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	Location           string     `json:"location"`
	StartTime          *time.Time `json:"startTime,omitempty"`
	EndTime            *time.Time `json:"endTime,omitempty"`
	CoverPolicyID      string     `json:"coverPolicyId,omitempty"`
	CoverObjectKey     string     `json:"coverObjectKey,omitempty"`
	CoverURL           string     `json:"coverUrl,omitempty"`
	IsPublic           bool       `json:"isPublic"`
	SubmissionEnabled  bool       `json:"submissionEnabled"`
	SubmissionPassword string     `json:"-"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type CreateEventRequest struct {
	Slug              string `json:"slug"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	Location          string `json:"location"`
	StartTime         string `json:"startTime"`
	EndTime           string `json:"endTime"`
	CoverPolicyID     string `json:"coverPolicyId"`
	CoverObjectKey    string `json:"coverObjectKey"`
	CoverURL          string `json:"coverUrl"`
	IsPublic          bool   `json:"isPublic"`
	SubmissionEnabled bool   `json:"submissionEnabled"`
	SubmissionPass    string `json:"submissionPassword"`
}
