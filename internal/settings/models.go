package settings

import "fluffcatch/internal/config"

const (
	KeyStoragePolicies = "storage_policies"
	KeyOIDC            = "oidc"
	KeySite            = "site"
)

type RuntimeSettings struct {
	StoragePolicies StoragePoliciesSettings `json:"storagePolicies"`
	OIDC            OIDCSettings            `json:"oidc"`
	Site            SiteSettings            `json:"site"`
}

type StoragePoliciesSettings struct {
	ActivePolicyID string          `json:"activePolicyId"`
	Policies       []StoragePolicy `json:"policies"`
}

type StoragePolicy struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Driver        string     `json:"driver"`
	LocalPath     string     `json:"localPath,omitempty"`
	PublicPrefix  string     `json:"publicPrefix"`
	PublicBaseURL string     `json:"publicBaseUrl,omitempty"`
	S3            S3Settings `json:"s3,omitempty"`
}

type S3Settings struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey,omitempty"`
	UseSSL    bool   `json:"useSsl"`
}

type OIDCSettings struct {
	Enabled      bool   `json:"enabled"`
	Provider     string `json:"provider"`
	IssuerURL    string `json:"issuerUrl"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret,omitempty"`
	RedirectURL  string `json:"redirectUrl"`
}

type SiteSettings struct {
	Name         string `json:"name"`
	Subtitle     string `json:"subtitle"`
	LogoURL      string `json:"logoUrl"`
	HomeMarkdown string `json:"homeMarkdown"`
}

const DefaultHomeMarkdown = ""

func FromConfig(cfg config.Config) RuntimeSettings {
	defaultPolicy := StoragePolicy{
		ID:            "default-local",
		Name:          "默认本地存储",
		Driver:        cfg.Storage.Driver,
		LocalPath:     cfg.Storage.LocalPath,
		PublicPrefix:  cfg.Storage.PublicPrefix,
		PublicBaseURL: cfg.Storage.PublicBaseURL,
		S3: S3Settings{
			Endpoint:  cfg.Storage.S3.Endpoint,
			Bucket:    cfg.Storage.S3.Bucket,
			Region:    cfg.Storage.S3.Region,
			AccessKey: cfg.Storage.S3.AccessKey,
			SecretKey: cfg.Storage.S3.SecretKey,
			UseSSL:    cfg.Storage.S3.UseSSL,
		},
	}

	return RuntimeSettings{
		StoragePolicies: StoragePoliciesSettings{
			ActivePolicyID: defaultPolicy.ID,
			Policies:       []StoragePolicy{defaultPolicy},
		},
		OIDC: OIDCSettings{
			Enabled:      cfg.OIDC.Enabled,
			Provider:     cfg.OIDC.Provider,
			IssuerURL:    cfg.OIDC.IssuerURL,
			ClientID:     cfg.OIDC.ClientID,
			ClientSecret: cfg.OIDC.ClientSecret,
			RedirectURL:  cfg.OIDC.RedirectURL,
		},
		Site: SiteSettings{
			Name:         cfg.App.Name,
			Subtitle:     "兽聚返图收集与画廊",
			LogoURL:      "",
			HomeMarkdown: DefaultHomeMarkdown,
		},
	}
}
