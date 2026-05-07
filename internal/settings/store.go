package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type Store struct {
	db       *sql.DB
	fallback RuntimeSettings
}

func NewStore(dbConn *sql.DB, fallback RuntimeSettings) *Store {
	return &Store{
		db:       dbConn,
		fallback: fallback,
	}
}

func (store *Store) Load(ctx context.Context) (RuntimeSettings, error) {
	current := store.fallback
	if store.db == nil {
		return current, nil
	}

	if err := store.loadValue(ctx, KeyStoragePolicies, &current.StoragePolicies); err != nil {
		return RuntimeSettings{}, err
	}

	if err := store.loadValue(ctx, KeyOIDC, &current.OIDC); err != nil {
		return RuntimeSettings{}, err
	}
	if err := store.loadValue(ctx, KeySite, &current.Site); err != nil {
		return RuntimeSettings{}, err
	}
	if err := store.loadValue(ctx, KeyUpload, &current.Upload); err != nil {
		return RuntimeSettings{}, err
	}

	return current, nil
}

func (store *Store) SaveSite(ctx context.Context, site SiteSettings) error {
	if store.db == nil {
		store.fallback.Site = site
		return nil
	}

	return store.saveValue(ctx, KeySite, site)
}

func (store *Store) SaveStoragePolicies(ctx context.Context, policies StoragePoliciesSettings) error {
	if store.db == nil {
		store.fallback.StoragePolicies = policies
		return nil
	}

	return store.saveValue(ctx, KeyStoragePolicies, policies)
}

func (store *Store) SaveOIDC(ctx context.Context, oidc OIDCSettings) error {
	if store.db == nil {
		store.fallback.OIDC = oidc
		return nil
	}

	return store.saveValue(ctx, KeyOIDC, oidc)
}

func (store *Store) SaveUpload(ctx context.Context, upload UploadSettings) error {
	if store.db == nil {
		store.fallback.Upload = upload
		return nil
	}

	return store.saveValue(ctx, KeyUpload, upload)
}

func (store *Store) loadValue(ctx context.Context, key string, target any) error {
	var raw json.RawMessage
	err := store.db.QueryRowContext(ctx, "SELECT `value` FROM settings WHERE `key` = ? LIMIT 1", key).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load setting %s: %w", key, err)
	}

	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode setting %s: %w", key, err)
	}

	return nil
}

func (store *Store) saveValue(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode setting %s: %w", key, err)
	}

	_, err = store.db.ExecContext(ctx, `
		INSERT INTO settings (`+"`key`, `value`"+`)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE
			`+"`value`"+` = VALUES(`+"`value`"+`),
			updated_at = CURRENT_TIMESTAMP
	`, key, string(raw))
	if err != nil {
		return fmt.Errorf("save setting %s: %w", key, err)
	}

	return nil
}
