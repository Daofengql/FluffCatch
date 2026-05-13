package settings

import (
	"fmt"
	"html"
	"time"

	"fluffcatch/internal/config"
)

const (
	KeyStoragePolicies = "storage_policies"
	KeySite            = "site"
	KeyUpload          = "upload"

	MaskedSecret = "***"
)

type RuntimeSettings struct {
	StoragePolicies StoragePoliciesSettings `json:"storagePolicies"`
	Site            SiteSettings            `json:"site"`
	Upload          UploadSettings          `json:"upload"`
}

func (s RuntimeSettings) Sanitize() RuntimeSettings {
	s.StoragePolicies = s.StoragePolicies.Sanitize()
	return s
}

type StoragePoliciesSettings struct {
	ActivePolicyID string          `json:"activePolicyId"`
	Policies       []StoragePolicy `json:"policies"`
}

func (s StoragePoliciesSettings) Sanitize() StoragePoliciesSettings {
	policies := s.Policies
	s.Policies = make([]StoragePolicy, len(policies))
	for i, p := range policies {
		s.Policies[i] = p.Sanitize()
	}
	return s
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

func (p StoragePolicy) Sanitize() StoragePolicy {
	p.S3 = p.S3.Sanitize()
	return p
}

type S3Settings struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey,omitempty"`
	UseSSL    bool   `json:"useSsl"`
	AccountID string `json:"accountId,omitempty"`
}

func (s S3Settings) Sanitize() S3Settings {
	if s.SecretKey != "" {
		s.SecretKey = MaskedSecret
	}
	return s
}

type SiteSettings struct {
	Name                       string          `json:"name"`
	Subtitle                   string          `json:"subtitle"`
	LogoURL                    string          `json:"logoUrl"`
	HomeMarkdown               string          `json:"homeMarkdown"`
	ThemeMode                  string          `json:"themeMode"`
	ThemePreset                string          `json:"themePreset"`
	ThemePrimaryColor          string          `json:"themePrimaryColor"`
	PublicBackgroundDesktopURL string          `json:"publicBackgroundDesktopUrl"`
	PublicBackgroundMobileURL  string          `json:"publicBackgroundMobileUrl"`
	FooterSections             []FooterSection `json:"footerSections"`
	ContactWidgetEnabled       bool            `json:"contactWidgetEnabled"`
	ContactWidgetTitle         string          `json:"contactWidgetTitle"`
	ContactWidgetHTML          string          `json:"contactWidgetHtml"`
}

type FooterSection struct {
	Title string `json:"title"`
	HTML  string `json:"html"`
}

type UploadSettings struct {
	MaxFileSizeMB        int `json:"maxFileSizeMb"`
	MaxVideoSizeMB       int `json:"maxVideoSizeMb"`
	MaxFilesPerUpload    int `json:"maxFilesPerUpload"`
	DefaultPageSize      int `json:"defaultPageSize"`
	MaxConcurrentUploads int `json:"maxConcurrentUploads"`
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
		Site: SiteSettings{
			Name:               cfg.App.Name,
			Subtitle:           "兽聚返图收集与画廊",
			HomeMarkdown:       DefaultHomeMarkdown,
			ThemeMode:          "system",
			ThemePreset:        "blue",
			ThemePrimaryColor:  "#2563eb",
			FooterSections:     defaultFooterSections(cfg.App.Name, "兽聚返图收集与画廊"),
			ContactWidgetTitle: "联系我",
		},
		Upload: UploadSettings{
			MaxFileSizeMB:        cfg.Upload.MaxSizeMB,
			MaxVideoSizeMB:       cfg.Upload.MaxVideoSizeMB,
			MaxFilesPerUpload:    cfg.Upload.MaxFilesPerUpload,
			DefaultPageSize:      cfg.Upload.DefaultPageSize,
			MaxConcurrentUploads: cfg.Upload.MaxConcurrentUploads,
		},
	}
}

func defaultFooterSections(siteName string, subtitle string) []FooterSection {
	if siteName == "" {
		siteName = "FluffCatch"
	}
	if subtitle == "" {
		subtitle = "兽聚返图收集与画廊"
	}

	return []FooterSection{
		{
			Title: "关于站点",
			HTML:  fmt.Sprintf("<p>%s</p><p>© %d %s. All rights reserved.</p>", html.EscapeString(subtitle), time.Now().Year(), html.EscapeString(siteName)),
		},
		{
			Title: "快速入口",
			HTML:  `<ul><li><a href="/">首页</a></li><li><a href="/submit">返图入口</a></li></ul>`,
		},
		{
			Title: "站点信息",
			HTML:  "<p>公开画廊、限时投稿和活动返图都会在这里汇总。</p>",
		},
	}
}
