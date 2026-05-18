package db

import "time"

type Setting struct {
	Key       string    `gorm:"column:key;primaryKey"`
	Value     []byte    `gorm:"column:value;type:json;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (Setting) TableName() string { return "settings" }

type Session struct {
	ID        string    `gorm:"column:id;primaryKey"`
	Username  string    `gorm:"column:username"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Session) TableName() string { return "sessions" }

type Event struct {
	ID                   int64      `gorm:"column:id;primaryKey;autoIncrement"`
	Title                string     `gorm:"column:title"`
	Description          string     `gorm:"column:description"`
	Location             string     `gorm:"column:location"`
	ProvinceCode         *string    `gorm:"column:province_code"`
	ProvinceName         *string    `gorm:"column:province_name"`
	CityCode             *string    `gorm:"column:city_code"`
	CityName             *string    `gorm:"column:city_name"`
	StartsAt             *time.Time `gorm:"column:starts_at"`
	EndsAt               *time.Time `gorm:"column:ends_at"`
	CoverStoragePolicyID *string    `gorm:"column:cover_storage_policy_id"`
	CoverObjectKey       *string    `gorm:"column:cover_object_key"`
	CoverThumbnailKey    *string    `gorm:"column:cover_thumbnail_key"`
	IsPublic             bool       `gorm:"column:is_public"`
	SubmissionEnabled    bool       `gorm:"column:submission_enabled"`
	PrivatePasswordHash  *string    `gorm:"column:private_password_hash"`
	PrivatePasswordPlain *string    `gorm:"column:private_password_plain"`
	SortAt               time.Time  `gorm:"column:sort_at"`
	CreatedAt            time.Time  `gorm:"column:created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
}

func (Event) TableName() string { return "events" }

type Photo struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement"`
	EventID          int64      `gorm:"column:event_id"`
	StoragePolicyID  string     `gorm:"column:storage_policy_id"`
	ObjectKey        string     `gorm:"column:object_key"`
	ThumbnailKey     *string    `gorm:"column:thumbnail_key"`
	ContentHash      string     `gorm:"column:content_hash"`
	ContentType      string     `gorm:"column:content_type"`
	SizeBytes        int64      `gorm:"column:size_bytes"`
	LikeCount        int64      `gorm:"column:like_count"`
	PhotographerName *string    `gorm:"column:photographer_name"`
	Visibility       string     `gorm:"column:visibility"`
	Exif             []byte     `gorm:"column:exif;type:json"`
	TakenAt          *time.Time `gorm:"column:taken_at"`
	SortAt           time.Time  `gorm:"column:sort_at"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (Photo) TableName() string { return "photos" }

type Submission struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement"`
	EventID          int64      `gorm:"column:event_id"`
	StoragePolicyID  string     `gorm:"column:storage_policy_id"`
	ObjectKey        string     `gorm:"column:object_key"`
	ThumbnailKey     *string    `gorm:"column:thumbnail_key"`
	ContentHash      string     `gorm:"column:content_hash"`
	ContentType      string     `gorm:"column:content_type"`
	SizeBytes        int64      `gorm:"column:size_bytes"`
	PhotographerName *string    `gorm:"column:photographer_name"`
	Tags             []byte     `gorm:"column:tags;type:json"`
	Status           string     `gorm:"column:status"`
	Exif             []byte     `gorm:"column:exif;type:json"`
	TakenAt          *time.Time `gorm:"column:taken_at"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	ApprovedAt       *time.Time `gorm:"column:approved_at"`
}

func (Submission) TableName() string { return "submissions" }

type Tag struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Name      string    `gorm:"column:name"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Tag) TableName() string { return "tags" }

type PhotoTag struct {
	PhotoID   int64     `gorm:"column:photo_id;primaryKey"`
	TagID     int64     `gorm:"column:tag_id;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (PhotoTag) TableName() string { return "photo_tags" }

type PhotoLike struct {
	PhotoID         int64     `gorm:"column:photo_id;primaryKey"`
	FingerprintHash string    `gorm:"column:fingerprint_hash;primaryKey"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (PhotoLike) TableName() string { return "photo_likes" }

type SubmissionLink struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement"`
	EventID          int64      `gorm:"column:event_id"`
	TokenHash        string     `gorm:"column:token_hash"`
	Label            string     `gorm:"column:label"`
	PhotographerName *string    `gorm:"column:photographer_name"`
	ExpiresAt        *time.Time `gorm:"column:expires_at"`
	MaxUses          int        `gorm:"column:max_uses"`
	UseCount         int        `gorm:"column:use_count"`
	RevokedAt        *time.Time `gorm:"column:revoked_at"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (SubmissionLink) TableName() string { return "submission_links" }
