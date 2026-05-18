package events

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fluffcatch/internal/auth"
	appdb "fluffcatch/internal/db"
	"fluffcatch/internal/storage"

	"gorm.io/gorm"
)

type Service struct {
	db             *gorm.DB
	storageManager *storage.Manager
}

func NewService(dbConn *gorm.DB, storageManager *storage.Manager) *Service {
	return &Service{db: dbConn, storageManager: storageManager}
}

func (service *Service) ListPublic(ctx context.Context) ([]Event, error) {
	page, err := service.ListPublicPage(ctx, ListOptions{})
	return page.Items, err
}

func (service *Service) ListPublicWithOptions(ctx context.Context, options ListOptions) ([]Event, error) {
	page, err := service.ListPublicPage(ctx, options)
	return page.Items, err
}

func (service *Service) ListPublicPage(ctx context.Context, options ListOptions) (Page, error) {
	return service.listPageWithOptions(ctx, true, false, options)
}

func (service *Service) ListAdmin(ctx context.Context) ([]Event, error) {
	return service.listWithOptions(ctx, false, true, ListOptions{})
}

func (service *Service) GetPublic(ctx context.Context, id int64) (Event, bool, error) {
	return service.get(ctx, id, true)
}

func (service *Service) GetAdmin(ctx context.Context, id int64) (Event, bool, error) {
	return service.getWithOptions(ctx, id, false, true)
}

func (service *Service) Create(ctx context.Context, req CreateEventRequest) (Event, error) {
	if service.db == nil {
		return Event{}, fmt.Errorf("database is required")
	}

	event, err := requestToEvent(req)
	if err != nil {
		return Event{}, err
	}

	privatePasswordHash, err := optionalPasswordHash(req.PrivatePassword)
	if err != nil {
		return Event{}, err
	}

	record := appdb.Event{
		Title:                event.Title,
		Description:          event.Description,
		Location:             event.Location,
		ProvinceCode:         stringPtr(event.ProvinceCode),
		ProvinceName:         stringPtr(event.ProvinceName),
		CityCode:             stringPtr(event.CityCode),
		CityName:             stringPtr(event.CityName),
		StartsAt:             event.StartTime,
		EndsAt:               event.EndTime,
		CoverStoragePolicyID: stringPtr(event.CoverPolicyID),
		CoverObjectKey:       stringPtr(event.CoverObjectKey),
		IsPublic:             event.IsPublic,
		SubmissionEnabled:    event.SubmissionEnabled,
		PrivatePasswordHash:  stringPtr(privatePasswordHash),
		PrivatePasswordPlain: stringPtr(req.PrivatePassword),
		SortAt:               eventSortTime(event.StartTime),
	}
	err = service.db.WithContext(ctx).Create(&record).Error
	if err != nil {
		return Event{}, friendlySQLError("create event", err)
	}

	created, found, err := service.GetAdmin(ctx, record.ID)
	if err != nil {
		return Event{}, err
	}
	if !found {
		return Event{}, fmt.Errorf("created event not found")
	}

	return created, nil
}

func (service *Service) Update(ctx context.Context, id int64, req CreateEventRequest) (Event, error) {
	if service.db == nil {
		return Event{}, fmt.Errorf("database is required")
	}

	event, err := requestToEvent(req)
	if err != nil {
		return Event{}, err
	}

	privatePasswordHash, err := optionalPasswordHash(req.PrivatePassword)
	if err != nil {
		return Event{}, err
	}

	var existing appdb.Event
	if err := service.db.WithContext(ctx).Where("id = ?", id).Take(&existing).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return Event{}, fmt.Errorf("event not found")
	} else if err != nil {
		return Event{}, fmt.Errorf("load event: %w", err)
	}

	sortAt := existing.CreatedAt
	if event.StartTime != nil {
		sortAt = *event.StartTime
	}
	updates := map[string]any{
		"title":              event.Title,
		"description":        event.Description,
		"location":           event.Location,
		"province_code":      stringPtr(event.ProvinceCode),
		"province_name":      stringPtr(event.ProvinceName),
		"city_code":          stringPtr(event.CityCode),
		"city_name":          stringPtr(event.CityName),
		"starts_at":          event.StartTime,
		"ends_at":            event.EndTime,
		"is_public":          event.IsPublic,
		"submission_enabled": event.SubmissionEnabled,
		"sort_at":            sortAt,
	}
	if req.RemoveCover || event.CoverPolicyID != "" || event.CoverObjectKey != "" {
		updates["cover_storage_policy_id"] = stringPtr(event.CoverPolicyID)
		updates["cover_object_key"] = stringPtr(event.CoverObjectKey)
	}
	if req.ClearPrivatePassword {
		updates["private_password_hash"] = nil
		updates["private_password_plain"] = nil
	}
	if privatePasswordHash != "" {
		updates["private_password_hash"] = privatePasswordHash
		updates["private_password_plain"] = strings.TrimSpace(req.PrivatePassword)
	}

	err = service.db.WithContext(ctx).Model(&appdb.Event{}).Where("id = ?", id).Updates(updates).Error
	if err != nil {
		return Event{}, friendlySQLError("update event", err)
	}

	updated, found, err := service.GetAdmin(ctx, id)
	if err != nil {
		return Event{}, err
	}
	if !found {
		return Event{}, fmt.Errorf("event not found")
	}

	return updated, nil
}

func (service *Service) Delete(ctx context.Context, id int64) (bool, []storage.StoredObject, error) {
	if service.db == nil {
		return false, nil, fmt.Errorf("database is required")
	}

	objects, err := service.objectsForEvent(ctx, id)
	if err != nil {
		return false, nil, err
	}

	result := service.db.WithContext(ctx).Where("id = ?", id).Delete(&appdb.Event{})
	if result.Error != nil {
		return false, nil, fmt.Errorf("delete event: %w", result.Error)
	}

	return result.RowsAffected > 0, objects, nil
}

func (service *Service) listWithOptions(ctx context.Context, onlyPublic bool, includePrivatePassword bool, options ListOptions) ([]Event, error) {
	page, err := service.listPageWithOptions(ctx, onlyPublic, includePrivatePassword, options)
	return page.Items, err
}

func (service *Service) listPageWithOptions(ctx context.Context, onlyPublic bool, includePrivatePassword bool, options ListOptions) (Page, error) {
	pageNumber, pageSize, paginated := normalizeListPagination(options)
	if service.db == nil {
		return Page{Items: []Event{}, Page: pageNumber, PageSize: pageSize}, nil
	}

	var records []appdb.Event
	query := service.db.WithContext(ctx).Model(&appdb.Event{})
	if onlyPublic {
		query = query.Where("is_public = ?", true)
	}
	if keyword := strings.TrimSpace(options.Query); keyword != "" {
		query = query.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}
	if cityCode := strings.TrimSpace(options.CityCode); cityCode != "" {
		query = query.Where("city_code = ?", cityCode)
	} else if provinceCode := strings.TrimSpace(options.ProvinceCode); provinceCode != "" {
		query = query.Where("province_code = ?", provinceCode)
	}
	if options.StartDate != nil {
		query = query.Where("COALESCE(ends_at, starts_at) >= ?", *options.StartDate)
	}
	if options.EndDate != nil {
		query = query.Where("COALESCE(starts_at, ends_at) <= ?", *options.EndDate)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return Page{}, fmt.Errorf("count events: %w", err)
	}

	query = applyEventListSort(query, options.Sort)
	if paginated {
		query = query.Limit(pageSize).Offset((pageNumber - 1) * pageSize)
	}
	if err := query.Find(&records).Error; err != nil {
		return Page{}, fmt.Errorf("list events: %w", err)
	}

	events := make([]Event, 0, len(records))
	for _, record := range records {
		event, err := service.eventFromRecord(ctx, record, includePrivatePassword)
		if err != nil {
			return Page{}, err
		}
		events = append(events, event)
	}

	if !paginated {
		pageSize = len(events)
	}
	totalPages := 0
	if pageSize > 0 && total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return Page{Items: events, Total: total, Page: pageNumber, PageSize: pageSize, TotalPages: totalPages}, nil
}

func normalizeListPagination(options ListOptions) (int, int, bool) {
	page := options.Page
	if page < 1 {
		page = 1
	}
	pageSize := options.PageSize
	if pageSize <= 0 {
		return page, 0, false
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize, true
}

func applyEventListSort(query *gorm.DB, sort string) *gorm.DB {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "start_asc", "time_asc", "date_asc", "oldest":
		return query.Order("sort_at ASC").Order("id ASC")
	default:
		return query.Order("sort_at DESC").Order("id DESC")
	}
}

func (service *Service) get(ctx context.Context, id int64, onlyPublic bool) (Event, bool, error) {
	return service.getWithOptions(ctx, id, onlyPublic, false)
}

func (service *Service) getWithOptions(ctx context.Context, id int64, onlyPublic bool, includePrivatePassword bool) (Event, bool, error) {
	if service.db == nil {
		return Event{}, false, nil
	}

	var record appdb.Event
	query := service.db.WithContext(ctx).Where("id = ?", id)
	if onlyPublic {
		query = query.Where("is_public = ?", true)
	}
	err := query.Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, err
	}

	event, err := service.eventFromRecord(ctx, record, includePrivatePassword)
	if err != nil {
		return Event{}, false, err
	}
	return event, true, nil
}

func (service *Service) objectsForEvent(ctx context.Context, id int64) ([]storage.StoredObject, error) {
	objects := []storage.StoredObject{}

	var event appdb.Event
	err := service.db.WithContext(ctx).Select("cover_storage_policy_id", "cover_object_key").Where("id = ?", id).Take(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return objects, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load event cover objects: %w", err)
	}
	if event.CoverStoragePolicyID != nil && event.CoverObjectKey != nil {
		objects = append(objects, storage.StoredObject{PolicyID: *event.CoverStoragePolicyID, Key: *event.CoverObjectKey})
	}

	if err := service.appendPhotoObjects(ctx, &objects, id); err != nil {
		return nil, err
	}
	if err := service.appendSubmissionObjects(ctx, &objects, id); err != nil {
		return nil, err
	}

	return objects, nil
}

func (service *Service) appendPhotoObjects(ctx context.Context, objects *[]storage.StoredObject, eventID int64) error {
	var photos []appdb.Photo
	if err := service.db.WithContext(ctx).Select("storage_policy_id", "object_key", "thumbnail_key").Where("event_id = ?", eventID).Find(&photos).Error; err != nil {
		return fmt.Errorf("load event photo objects: %w", err)
	}
	for _, photo := range photos {
		*objects = append(*objects, storage.StoredObject{PolicyID: photo.StoragePolicyID, Key: photo.ObjectKey})
		if photo.ThumbnailKey != nil && strings.TrimSpace(*photo.ThumbnailKey) != "" {
			*objects = append(*objects, storage.StoredObject{PolicyID: photo.StoragePolicyID, Key: *photo.ThumbnailKey})
		}
	}
	return nil
}

func (service *Service) appendSubmissionObjects(ctx context.Context, objects *[]storage.StoredObject, eventID int64) error {
	var submissions []appdb.Submission
	if err := service.db.WithContext(ctx).Select("storage_policy_id", "object_key", "thumbnail_key").Where("event_id = ?", eventID).Find(&submissions).Error; err != nil {
		return fmt.Errorf("load event submission objects: %w", err)
	}
	for _, submission := range submissions {
		*objects = append(*objects, storage.StoredObject{PolicyID: submission.StoragePolicyID, Key: submission.ObjectKey})
		if submission.ThumbnailKey != nil && strings.TrimSpace(*submission.ThumbnailKey) != "" {
			*objects = append(*objects, storage.StoredObject{PolicyID: submission.StoragePolicyID, Key: *submission.ThumbnailKey})
		}
	}
	return nil
}

func (service *Service) eventFromRecord(ctx context.Context, record appdb.Event, includePrivatePassword bool) (Event, error) {
	event := Event{
		ID:                record.ID,
		Title:             record.Title,
		Description:       record.Description,
		Location:          record.Location,
		ProvinceCode:      stringValue(record.ProvinceCode),
		ProvinceName:      stringValue(record.ProvinceName),
		CityCode:          stringValue(record.CityCode),
		CityName:          stringValue(record.CityName),
		StartTime:         record.StartsAt,
		EndTime:           record.EndsAt,
		CoverPolicyID:     stringValue(record.CoverStoragePolicyID),
		CoverObjectKey:    stringValue(record.CoverObjectKey),
		IsPublic:          record.IsPublic,
		SubmissionEnabled: record.SubmissionEnabled,
		CreatedAt:         record.CreatedAt,
		UpdatedAt:         record.UpdatedAt,
	}
	if includePrivatePassword {
		event.PrivatePassword = stringValue(record.PrivatePasswordPlain)
	}

	countQuery := service.db.WithContext(ctx).Model(&appdb.Photo{}).Where("event_id = ?", record.ID)
	if !includePrivatePassword {
		countQuery = countQuery.Where("visibility = ?", "public")
	}
	if err := countQuery.Count(&event.PhotoCount).Error; err != nil {
		return Event{}, fmt.Errorf("count event photos: %w", err)
	}

	if event.CoverPolicyID != "" && event.CoverObjectKey != "" {
		if store, err := service.storageManager.StoreForPolicy(event.CoverPolicyID); err == nil {
			event.CoverURL = store.PublicURL(event.CoverObjectKey)
		} else {
			event.CoverURL = storage.MediaURL(event.CoverPolicyID, event.CoverObjectKey)
		}
	}

	return event, nil
}

func requestToEvent(req CreateEventRequest) (Event, error) {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return Event{}, fmt.Errorf("title is required")
	}
	if len(req.Title) > 200 {
		return Event{}, fmt.Errorf("title must be 200 characters or fewer")
	}

	req.Description = strings.TrimSpace(req.Description)
	if len(req.Description) > 10000 {
		return Event{}, fmt.Errorf("description must be 10000 characters or fewer")
	}
	req.Location = strings.TrimSpace(req.Location)
	if len(req.Location) > 500 {
		return Event{}, fmt.Errorf("location must be 500 characters or fewer")
	}

	req.ProvinceCode = strings.TrimSpace(req.ProvinceCode)
	req.ProvinceName = strings.TrimSpace(req.ProvinceName)
	req.CityCode = strings.TrimSpace(req.CityCode)
	req.CityName = strings.TrimSpace(req.CityName)
	if len(req.ProvinceCode) > 20 || len(req.CityCode) > 20 {
		return Event{}, fmt.Errorf("province/city code is too long")
	}

	startTime, err := parseOptionalTime(req.StartTime)
	if err != nil {
		return Event{}, fmt.Errorf("invalid startTime: %w", err)
	}
	endTime, err := parseOptionalTime(req.EndTime)
	if err != nil {
		return Event{}, fmt.Errorf("invalid endTime: %w", err)
	}

	return Event{
		Title:             req.Title,
		Description:       req.Description,
		Location:          req.Location,
		ProvinceCode:      req.ProvinceCode,
		ProvinceName:      req.ProvinceName,
		CityCode:          req.CityCode,
		CityName:          req.CityName,
		StartTime:         startTime,
		EndTime:           endTime,
		CoverPolicyID:     strings.TrimSpace(req.CoverPolicyID),
		CoverObjectKey:    strings.TrimSpace(req.CoverObjectKey),
		CoverURL:          strings.TrimSpace(req.CoverURL),
		IsPublic:          req.IsPublic,
		SubmissionEnabled: req.SubmissionEnabled,
	}, nil
}

func friendlySQLError(action string, err error) error {
	return fmt.Errorf("%s: %w", action, err)
}

func parseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	formats := []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02"}
	for _, format := range formats {
		parsed, err := time.ParseInLocation(format, value, time.Local)
		if err == nil {
			return &parsed, nil
		}
	}

	return nil, fmt.Errorf("unsupported time format")
}

func optionalPasswordHash(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", nil
	}

	return auth.HashPassword(password)
}

func eventSortTime(startTime *time.Time) time.Time {
	if startTime != nil {
		return *startTime
	}
	return time.Now()
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
