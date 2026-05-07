package events

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"fluffcatch/internal/auth"
	"fluffcatch/internal/storage"
)

type Service struct {
	db             *sql.DB
	storageManager *storage.Manager
}

func NewService(dbConn *sql.DB, storageManager *storage.Manager) *Service {
	return &Service{db: dbConn, storageManager: storageManager}
}

func (service *Service) ListPublic(ctx context.Context) ([]Event, error) {
	return service.list(ctx, "WHERE is_public = true")
}

func (service *Service) ListAdmin(ctx context.Context) ([]Event, error) {
	return service.listWithOptions(ctx, "", true)
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

	passwordHash, err := optionalPasswordHash(req.SubmissionPass)
	if err != nil {
		return Event{}, err
	}
	privatePasswordHash, err := optionalPasswordHash(req.PrivatePassword)
	if err != nil {
		return Event{}, err
	}

	result, err := service.db.ExecContext(ctx, `
		INSERT INTO events (
			title, description, location, province_code, province_name, city_code, city_name, starts_at, ends_at,
			cover_storage_policy_id, cover_object_key,
			is_public, submission_enabled, submission_password_hash, submission_password_plain,
			private_password_hash, private_password_plain, sort_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.Title, event.Description, event.Location, nullableString(event.ProvinceCode), nullableString(event.ProvinceName), nullableString(event.CityCode), nullableString(event.CityName), nullableTime(event.StartTime), nullableTime(event.EndTime), nullableString(event.CoverPolicyID), nullableString(event.CoverObjectKey), event.IsPublic, event.SubmissionEnabled, nullableString(passwordHash), nullableString(req.SubmissionPass), nullableString(privatePasswordHash), nullableString(req.PrivatePassword), eventSortTime(event.StartTime))
	if err != nil {
		return Event{}, friendlySQLError("create event", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Event{}, fmt.Errorf("read event id: %w", err)
	}

	created, found, err := service.GetAdmin(ctx, id)
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

	passwordHash, err := optionalPasswordHash(req.SubmissionPass)
	if err != nil {
		return Event{}, err
	}
	privatePasswordHash, err := optionalPasswordHash(req.PrivatePassword)
	if err != nil {
		return Event{}, err
	}

	if req.RemoveCover || event.CoverPolicyID != "" || event.CoverObjectKey != "" {
		_, err = service.db.ExecContext(ctx, `
			UPDATE events
			SET title = ?, description = ?, location = ?, province_code = ?, province_name = ?, city_code = ?, city_name = ?, starts_at = ?, ends_at = ?,
				cover_storage_policy_id = ?, cover_object_key = ?,
				is_public = ?, submission_enabled = ?,
				submission_password_hash = COALESCE(NULLIF(?, ''), submission_password_hash),
				submission_password_plain = COALESCE(NULLIF(?, ''), submission_password_plain),
				private_password_hash = COALESCE(NULLIF(?, ''), private_password_hash),
				private_password_plain = COALESCE(NULLIF(?, ''), private_password_plain),
				sort_at = COALESCE(?, created_at)
			WHERE id = ?
		`, event.Title, event.Description, event.Location, nullableString(event.ProvinceCode), nullableString(event.ProvinceName), nullableString(event.CityCode), nullableString(event.CityName), nullableTime(event.StartTime), nullableTime(event.EndTime), nullableString(event.CoverPolicyID), nullableString(event.CoverObjectKey), event.IsPublic, event.SubmissionEnabled, nullableString(passwordHash), nullableString(req.SubmissionPass), nullableString(privatePasswordHash), nullableString(req.PrivatePassword), nullableTime(event.StartTime), id)
	} else {
		_, err = service.db.ExecContext(ctx, `
			UPDATE events
			SET title = ?, description = ?, location = ?, province_code = ?, province_name = ?, city_code = ?, city_name = ?, starts_at = ?, ends_at = ?,
				is_public = ?, submission_enabled = ?,
				submission_password_hash = COALESCE(NULLIF(?, ''), submission_password_hash),
				submission_password_plain = COALESCE(NULLIF(?, ''), submission_password_plain),
				private_password_hash = COALESCE(NULLIF(?, ''), private_password_hash),
				private_password_plain = COALESCE(NULLIF(?, ''), private_password_plain),
				sort_at = COALESCE(?, created_at)
			WHERE id = ?
		`, event.Title, event.Description, event.Location, nullableString(event.ProvinceCode), nullableString(event.ProvinceName), nullableString(event.CityCode), nullableString(event.CityName), nullableTime(event.StartTime), nullableTime(event.EndTime), event.IsPublic, event.SubmissionEnabled, nullableString(passwordHash), nullableString(req.SubmissionPass), nullableString(privatePasswordHash), nullableString(req.PrivatePassword), nullableTime(event.StartTime), id)
	}
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

	result, err := service.db.ExecContext(ctx, "DELETE FROM events WHERE id = ?", id)
	if err != nil {
		return false, nil, fmt.Errorf("delete event: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return true, objects, nil
	}
	return affected > 0, objects, nil
}

func (service *Service) SubmissionPasswordHash(ctx context.Context, eventID int64) (string, bool, bool, error) {
	var enabled bool
	var passwordHash sql.NullString
	err := service.db.QueryRowContext(ctx, "SELECT submission_enabled, submission_password_hash FROM events WHERE id = ? LIMIT 1", eventID).Scan(&enabled, &passwordHash)
	if err == sql.ErrNoRows {
		return "", false, false, nil
	}
	if err != nil {
		return "", false, false, fmt.Errorf("load submission password: %w", err)
	}

	return passwordHash.String, passwordHash.Valid, enabled, nil
}

func (service *Service) list(ctx context.Context, where string) ([]Event, error) {
	return service.listWithOptions(ctx, where, false)
}

func (service *Service) listWithOptions(ctx context.Context, where string, includeSubmissionPassword bool) ([]Event, error) {
	if service.db == nil {
		return []Event{}, nil
	}

	query := `
		SELECT id, title, description, location,
			COALESCE(province_code, ''), COALESCE(province_name, ''), COALESCE(city_code, ''), COALESCE(city_name, ''),
			starts_at, ends_at,
			cover_storage_policy_id, cover_object_key, is_public, submission_enabled,
			(SELECT COUNT(*) FROM photos WHERE photos.event_id = events.id`
	if !includeSubmissionPassword {
		query += ` AND photos.visibility = 'public'`
	}
	query += `), `
	if includeSubmissionPassword {
		query += `COALESCE(submission_password_plain, ''), COALESCE(private_password_plain, ''), `
	} else {
		query += `'', '', `
	}
	query += `created_at, updated_at
		FROM events
		` + where + `
		ORDER BY sort_at DESC, id DESC
	`

	rows, err := service.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		event, err := service.scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}

func (service *Service) get(ctx context.Context, id int64, onlyPublic bool) (Event, bool, error) {
	return service.getWithOptions(ctx, id, onlyPublic, false)
}

func (service *Service) getWithOptions(ctx context.Context, id int64, onlyPublic bool, includeSubmissionPassword bool) (Event, bool, error) {
	if service.db == nil {
		return Event{}, false, nil
	}

	query := `
		SELECT id, title, description, location,
			COALESCE(province_code, ''), COALESCE(province_name, ''), COALESCE(city_code, ''), COALESCE(city_name, ''),
			starts_at, ends_at,
			cover_storage_policy_id, cover_object_key, is_public, submission_enabled,
			(SELECT COUNT(*) FROM photos WHERE photos.event_id = events.id`
	if includeSubmissionPassword {
		query += `), `
	} else {
		query += ` AND photos.visibility = 'public'), `
	}
	if includeSubmissionPassword {
		query += `COALESCE(submission_password_plain, ''), COALESCE(private_password_plain, ''), `
	} else {
		query += `'', '', `
	}
	query += `created_at, updated_at
		FROM events
		WHERE id = ?
	`
	args := []any{id}
	if onlyPublic {
		query += " AND is_public = true"
	}
	query += " LIMIT 1"

	row := service.db.QueryRowContext(ctx, query, args...)
	event, err := service.scanEvent(row)
	if err == sql.ErrNoRows {
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, err
	}

	return event, true, nil
}

func (service *Service) objectsForEvent(ctx context.Context, id int64) ([]storage.StoredObject, error) {
	objects := []storage.StoredObject{}

	var coverPolicyID sql.NullString
	var coverObjectKey sql.NullString
	err := service.db.QueryRowContext(ctx, `
		SELECT cover_storage_policy_id, cover_object_key
		FROM events
		WHERE id = ?
		LIMIT 1
	`, id).Scan(&coverPolicyID, &coverObjectKey)
	if err == sql.ErrNoRows {
		return objects, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load event cover objects: %w", err)
	}
	if coverPolicyID.Valid && coverObjectKey.Valid {
		objects = append(objects, storage.StoredObject{PolicyID: coverPolicyID.String, Key: coverObjectKey.String})
	}

	if err := service.appendEventObjects(ctx, &objects, "SELECT storage_policy_id, object_key, COALESCE(thumbnail_key, '') FROM photos WHERE event_id = ?", id); err != nil {
		return nil, err
	}
	if err := service.appendEventObjects(ctx, &objects, "SELECT storage_policy_id, object_key, COALESCE(thumbnail_key, '') FROM submissions WHERE event_id = ?", id); err != nil {
		return nil, err
	}

	return objects, nil
}

func (service *Service) appendEventObjects(ctx context.Context, objects *[]storage.StoredObject, query string, args ...any) error {
	rows, err := service.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("load event objects: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var policyID string
		var objectKey string
		var thumbnailKey string
		if err := rows.Scan(&policyID, &objectKey, &thumbnailKey); err != nil {
			return fmt.Errorf("scan event object: %w", err)
		}
		*objects = append(*objects, storage.StoredObject{PolicyID: policyID, Key: objectKey})
		if strings.TrimSpace(thumbnailKey) != "" {
			*objects = append(*objects, storage.StoredObject{PolicyID: policyID, Key: thumbnailKey})
		}
	}

	return rows.Err()
}

type eventScanner interface {
	Scan(dest ...any) error
}

func (service *Service) scanEvent(scanner eventScanner) (Event, error) {
	var event Event
	var startsAt sql.NullTime
	var endsAt sql.NullTime
	var coverPolicyID sql.NullString
	var coverObjectKey sql.NullString

	if err := scanner.Scan(
		&event.ID,
		&event.Title,
		&event.Description,
		&event.Location,
		&event.ProvinceCode,
		&event.ProvinceName,
		&event.CityCode,
		&event.CityName,
		&startsAt,
		&endsAt,
		&coverPolicyID,
		&coverObjectKey,
		&event.IsPublic,
		&event.SubmissionEnabled,
		&event.PhotoCount,
		&event.SubmissionPassword,
		&event.PrivatePassword,
		&event.CreatedAt,
		&event.UpdatedAt,
	); err != nil {
		return Event{}, err
	}

	if startsAt.Valid {
		event.StartTime = &startsAt.Time
	}
	if endsAt.Valid {
		event.EndTime = &endsAt.Time
	}
	if coverPolicyID.Valid {
		event.CoverPolicyID = coverPolicyID.String
	}
	if coverObjectKey.Valid {
		event.CoverObjectKey = coverObjectKey.String
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

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func eventSortTime(startTime *time.Time) time.Time {
	if startTime != nil {
		return *startTime
	}
	return time.Now()
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
