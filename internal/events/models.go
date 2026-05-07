package events

import "time"

type Event struct {
	ID                 int64      `json:"id"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	Location           string     `json:"location"`
	ProvinceCode       string     `json:"provinceCode,omitempty"`
	ProvinceName       string     `json:"provinceName,omitempty"`
	CityCode           string     `json:"cityCode,omitempty"`
	CityName           string     `json:"cityName,omitempty"`
	StartTime          *time.Time `json:"startTime,omitempty"`
	EndTime            *time.Time `json:"endTime,omitempty"`
	CoverPolicyID      string     `json:"coverPolicyId,omitempty"`
	CoverObjectKey     string     `json:"coverObjectKey,omitempty"`
	CoverURL           string     `json:"coverUrl,omitempty"`
	IsPublic           bool       `json:"isPublic"`
	SubmissionEnabled  bool       `json:"submissionEnabled"`
	SubmissionPassword string     `json:"submissionPassword,omitempty"`
	PrivatePassword    string     `json:"privatePassword,omitempty"`
	PhotoCount         int64      `json:"photoCount"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type CreateEventRequest struct {
	Title             string `json:"title"`
	Description       string `json:"description"`
	Location          string `json:"location"`
	ProvinceCode      string `json:"provinceCode"`
	ProvinceName      string `json:"provinceName"`
	CityCode          string `json:"cityCode"`
	CityName          string `json:"cityName"`
	StartTime         string `json:"startTime"`
	EndTime           string `json:"endTime"`
	CoverPolicyID     string `json:"coverPolicyId"`
	CoverObjectKey    string `json:"coverObjectKey"`
	CoverURL          string `json:"coverUrl"`
	RemoveCover       bool   `json:"removeCover"`
	IsPublic          bool   `json:"isPublic"`
	SubmissionEnabled bool   `json:"submissionEnabled"`
	SubmissionPass    string `json:"submissionPassword"`
	PrivatePassword   string `json:"privatePassword"`
}
