package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
)

type Object struct {
	Key         string
	Content     io.Reader
	ContentType string
	Size        int64
}

type StoredObject struct {
	Key      string `json:"key"`
	URL      string `json:"url"`
	PolicyID string `json:"policyId"`
}

type Store interface {
	Put(ctx context.Context, object Object) (StoredObject, error)
	Delete(ctx context.Context, key string) error
	PublicURL(key string) string
}

type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	UseSSL    bool
	AccountID string
}

type Config struct {
	PolicyID      string
	Name          string
	Driver        string
	LocalPath     string
	PublicPrefix  string
	PublicBaseURL string
	S3            S3Config
}

type PolicyStore struct {
	policyID string
	driver   string
	baseURL  string
	store    Store
}

func (store PolicyStore) Put(ctx context.Context, object Object) (StoredObject, error) {
	stored, err := store.store.Put(ctx, object)
	if err != nil {
		return StoredObject{}, err
	}

	stored.PolicyID = store.policyID
	stored.URL = store.PublicURL(stored.Key)
	return stored, nil
}

func (store PolicyStore) Delete(ctx context.Context, key string) error {
	return store.store.Delete(ctx, key)
}

func (store PolicyStore) PublicURL(key string) string {
	if store.baseURL != "" {
		return DirectURL(store.baseURL, key)
	}

	return MediaURL(store.policyID, key)
}

func MediaURL(policyID string, key string) string {
	escapedPolicy := url.PathEscape(policyID)
	escapedKey := strings.ReplaceAll(url.PathEscape(key), "%2F", "/")
	return "/media/" + escapedPolicy + "/" + escapedKey
}

func DirectURL(baseURL string, key string) string {
	escapedKey := strings.ReplaceAll(url.PathEscape(key), "%2F", "/")
	return strings.TrimRight(baseURL, "/") + "/" + escapedKey
}

func New(config Config) (Store, error) {
	switch config.Driver {
	case "local":
		return PolicyStore{
			policyID: config.PolicyID,
			driver:   config.Driver,
			baseURL:  config.PublicBaseURL,
			store:    NewLocalStore(config.LocalPath, config.PublicPrefix),
		}, nil
	case "aws-s3", "minio", "cf-r2", "s3":
		if config.PublicBaseURL == "" {
			return nil, fmt.Errorf("publicBaseUrl is required for %s storage", config.Driver)
		}
		s3Store, err := NewS3Store(config.S3, config.PublicBaseURL)
		if err != nil {
			return nil, err
		}
		return PolicyStore{
			policyID: config.PolicyID,
			driver:   config.Driver,
			baseURL:  config.PublicBaseURL,
			store:    s3Store,
		}, nil
	case "aliyun-oss":
		if config.PublicBaseURL == "" {
			return nil, fmt.Errorf("publicBaseUrl is required for aliyun-oss storage")
		}
		ossStore, err := NewOSSStore(config.S3, config.PublicBaseURL)
		if err != nil {
			return nil, err
		}
		return PolicyStore{
			policyID: config.PolicyID,
			driver:   config.Driver,
			baseURL:  config.PublicBaseURL,
			store:    ossStore,
		}, nil
	case "tencent-cos":
		if config.PublicBaseURL == "" {
			return nil, fmt.Errorf("publicBaseUrl is required for tencent-cos storage")
		}
		cosStore, err := NewCOSStore(config.S3, config.PublicBaseURL)
		if err != nil {
			return nil, err
		}
		return PolicyStore{
			policyID: config.PolicyID,
			driver:   config.Driver,
			baseURL:  config.PublicBaseURL,
			store:    cosStore,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage driver %q", config.Driver)
	}
}

type Manager struct {
	mu             sync.RWMutex
	activePolicyID string
	configs        map[string]Config
	stores         map[string]Store
}

func NewManager(activePolicyID string, configs []Config) (*Manager, error) {
	manager := &Manager{}
	if err := manager.Reconfigure(activePolicyID, configs); err != nil {
		return nil, err
	}

	return manager, nil
}

func (manager *Manager) ActivePolicyID() string {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.activePolicyID
}

func (manager *Manager) ActiveStore() (Store, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	store, ok := manager.stores[manager.activePolicyID]
	if !ok {
		return nil, fmt.Errorf("active storage policy %q is unavailable", manager.activePolicyID)
	}

	return store, nil
}

func (manager *Manager) StoreForPolicy(policyID string) (Store, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	store, ok := manager.stores[policyID]
	if !ok {
		return nil, fmt.Errorf("storage policy %q is unavailable", policyID)
	}

	return store, nil
}

func (manager *Manager) ConfigForPolicy(policyID string) (Config, bool) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	config, ok := manager.configs[policyID]
	return config, ok
}

func (manager *Manager) ActiveConfig() (Config, bool) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	config, ok := manager.configs[manager.activePolicyID]
	return config, ok
}

func (manager *Manager) Reconfigure(activePolicyID string, configs []Config) error {
	if len(configs) == 0 {
		return fmt.Errorf("at least one storage policy is required")
	}

	nextConfigs := map[string]Config{}
	nextStores := map[string]Store{}
	activeFound := false

	for _, config := range configs {
		if config.PolicyID == "" {
			return fmt.Errorf("storage policy id is required")
		}
		if _, exists := nextConfigs[config.PolicyID]; exists {
			return fmt.Errorf("duplicate storage policy id %q", config.PolicyID)
		}

		store, err := New(config)
		if err != nil {
			return err
		}

		if config.PolicyID == activePolicyID {
			activeFound = true
		}

		nextConfigs[config.PolicyID] = config
		nextStores[config.PolicyID] = store
	}

	if activePolicyID == "" {
		activePolicyID = configs[0].PolicyID
		activeFound = true
	}

	if !activeFound {
		return fmt.Errorf("active storage policy %q is unavailable", activePolicyID)
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.activePolicyID = activePolicyID
	manager.configs = nextConfigs
	manager.stores = nextStores
	return nil
}
