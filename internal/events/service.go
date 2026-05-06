package events

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"fluffcatch/internal/auth"
	"fluffcatch/internal/storage"
)

type Service struct {
	db *sql.DB
}

func NewService(dbConn *sql.DB) *Service {
	return &Service{db: dbConn}
}

func (service *Service) ListPublic(ctx context.Context) ([]Event, error) {
	return service.list(ctx, "WHERE is_public = true")
}

func (service *Service) ListAdmin(ctx context.Context) ([]Event, error) {
	return service.list(ctx, "")
}

func (service *Service) GetPublic(ctx context.Context, id int64) (Event, bool, error) {
	return service.get(ctx, id, true)
}

func (service *Service) GetAdmin(ctx context.Context, id int64) (Event, bool, error) {
	return service.get(ctx, id, false)
}

func (service *Service) Create(ctx context.Context, req CreateEventRequest) (Event, error) {
	if service.db == nil {
		return Event{}, fmt.Errorf("database is required")
	}

	event, err := requestToEvent(req)
	if err != nil {
		return Event{}, err
	}
	event.Slug, err = service.uniqueSlug(ctx, event.Slug, event.Title, 0)
	if err != nil {
		return Event{}, err
	}

	passwordHash, err := optionalPasswordHash(req.SubmissionPass)
	if err != nil {
		return Event{}, err
	}

	result, err := service.db.ExecContext(ctx, `
		INSERT INTO events (
			slug, title, description, location, starts_at, ends_at,
			cover_storage_policy_id, cover_object_key,
			is_public, submission_enabled, submission_password_hash
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.Slug, event.Title, event.Description, event.Location, nullableTime(event.StartTime), nullableTime(event.EndTime), nullableString(event.CoverPolicyID), nullableString(event.CoverObjectKey), event.IsPublic, event.SubmissionEnabled, nullableString(passwordHash))
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
	event.Slug, err = service.uniqueSlug(ctx, event.Slug, event.Title, id)
	if err != nil {
		return Event{}, err
	}

	passwordHash, err := optionalPasswordHash(req.SubmissionPass)
	if err != nil {
		return Event{}, err
	}

	if req.SubmissionPass == "" {
		_, err = service.db.ExecContext(ctx, `
			UPDATE events
			SET slug = ?, title = ?, description = ?, location = ?, starts_at = ?, ends_at = ?,
				cover_storage_policy_id = ?, cover_object_key = ?,
				is_public = ?, submission_enabled = ?
			WHERE id = ?
		`, event.Slug, event.Title, event.Description, event.Location, nullableTime(event.StartTime), nullableTime(event.EndTime), nullableString(event.CoverPolicyID), nullableString(event.CoverObjectKey), event.IsPublic, event.SubmissionEnabled, id)
	} else {
		_, err = service.db.ExecContext(ctx, `
			UPDATE events
			SET slug = ?, title = ?, description = ?, location = ?, starts_at = ?, ends_at = ?,
				cover_storage_policy_id = ?, cover_object_key = ?,
				is_public = ?, submission_enabled = ?, submission_password_hash = ?
			WHERE id = ?
		`, event.Slug, event.Title, event.Description, event.Location, nullableTime(event.StartTime), nullableTime(event.EndTime), nullableString(event.CoverPolicyID), nullableString(event.CoverObjectKey), event.IsPublic, event.SubmissionEnabled, nullableString(passwordHash), id)
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
	if service.db == nil {
		return []Event{}, nil
	}

	query := `
		SELECT id, slug, title, description, location, starts_at, ends_at,
			cover_storage_policy_id, cover_object_key, is_public, submission_enabled, created_at, updated_at
		FROM events
		` + where + `
		ORDER BY COALESCE(starts_at, created_at) DESC, id DESC
	`

	rows, err := service.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		event, err := scanEvent(rows)
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
	if service.db == nil {
		return Event{}, false, nil
	}

	query := `
		SELECT id, slug, title, description, location, starts_at, ends_at,
			cover_storage_policy_id, cover_object_key, is_public, submission_enabled, created_at, updated_at
		FROM events
		WHERE id = ?
	`
	args := []any{id}
	if onlyPublic {
		query += " AND is_public = true"
	}
	query += " LIMIT 1"

	row := service.db.QueryRowContext(ctx, query, args...)
	event, err := scanEvent(row)
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

func scanEvent(scanner eventScanner) (Event, error) {
	var event Event
	var startsAt sql.NullTime
	var endsAt sql.NullTime
	var coverPolicyID sql.NullString
	var coverObjectKey sql.NullString

	if err := scanner.Scan(
		&event.ID,
		&event.Slug,
		&event.Title,
		&event.Description,
		&event.Location,
		&startsAt,
		&endsAt,
		&coverPolicyID,
		&coverObjectKey,
		&event.IsPublic,
		&event.SubmissionEnabled,
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
		event.CoverURL = storage.MediaURL(event.CoverPolicyID, event.CoverObjectKey)
	}

	return event, nil
}

func requestToEvent(req CreateEventRequest) (Event, error) {
	if strings.TrimSpace(req.Title) == "" {
		return Event{}, fmt.Errorf("title is required")
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
		Slug:              normalizeSlug(req.Slug),
		Title:             strings.TrimSpace(req.Title),
		Description:       strings.TrimSpace(req.Description),
		Location:          strings.TrimSpace(req.Location),
		StartTime:         startTime,
		EndTime:           endTime,
		CoverPolicyID:     strings.TrimSpace(req.CoverPolicyID),
		CoverObjectKey:    strings.TrimSpace(req.CoverObjectKey),
		CoverURL:          strings.TrimSpace(req.CoverURL),
		IsPublic:          req.IsPublic,
		SubmissionEnabled: req.SubmissionEnabled,
	}, nil
}

func (service *Service) uniqueSlug(ctx context.Context, requested string, title string, excludeID int64) (string, error) {
	base := normalizeSlug(requested)
	if base == "" {
		base = normalizeSlug(title)
	}
	if base == "" {
		base = "event"
	}

	for index := 0; index < 100; index++ {
		candidate := base
		if index > 0 {
			candidate = base + "-" + strconv.Itoa(index+1)
		}

		available, err := service.slugAvailable(ctx, candidate, excludeID)
		if err != nil {
			return "", err
		}
		if available {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not generate a unique slug")
}

func (service *Service) slugAvailable(ctx context.Context, slug string, excludeID int64) (bool, error) {
	var id int64
	err := service.db.QueryRowContext(ctx, "SELECT id FROM events WHERE slug = ? LIMIT 1", slug).Scan(&id)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("check slug availability: %w", err)
	}
	return id == excludeID, nil
}

var slugSeparators = regexp.MustCompile(`[^a-zA-Z0-9\p{Han}]+`)

func normalizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = slugSeparators.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if len([]rune(value)) <= 80 {
		return value
	}

	runes := []rune(value)
	return strings.Trim(string(runes[:80]), "-")
}

func friendlySQLError(action string, err error) error {
	if strings.Contains(err.Error(), "events_slug_unique") || strings.Contains(err.Error(), "Duplicate entry") {
		return fmt.Errorf("%s: slug already exists; please use a different URL identifier", action)
	}
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

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
