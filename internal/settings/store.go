package settings

import (
	"context"
	"encoding/json"
	"fmt"

	appdb "fluffcatch/internal/db"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db       *gorm.DB
	fallback RuntimeSettings
}

func NewStore(dbConn *gorm.DB, fallback RuntimeSettings) *Store {
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

func (store *Store) SaveUpload(ctx context.Context, upload UploadSettings) error {
	if store.db == nil {
		store.fallback.Upload = upload
		return nil
	}

	return store.saveValue(ctx, KeyUpload, upload)
}

func (store *Store) loadValue(ctx context.Context, key string, target any) error {
	var setting appdb.Setting
	result := store.db.WithContext(ctx).Where("`key` = ?", key).Limit(1).Find(&setting)
	if result.Error != nil {
		return fmt.Errorf("load setting %s: %w", key, result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}

	if err := json.Unmarshal(setting.Value, target); err != nil {
		return fmt.Errorf("decode setting %s: %w", key, err)
	}

	return nil
}

func (store *Store) saveValue(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode setting %s: %w", key, err)
	}

	result := store.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"value":      raw,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		}),
	}).Create(&appdb.Setting{Key: key, Value: raw})
	if result.Error != nil {
		return fmt.Errorf("save setting %s: %w", key, result.Error)
	}

	return nil
}
