package settings

import (
	"context"
	"testing"
)

func TestStoreFallsBackWithoutDatabase(t *testing.T) {
	fallback := RuntimeSettings{
		StoragePolicies: StoragePoliciesSettings{
			ActivePolicyID: "default-local",
			Policies: []StoragePolicy{
				{
					ID:           "default-local",
					Name:         "默认本地存储",
					Driver:       "local",
					LocalPath:    "data/uploads",
					PublicPrefix: "/media",
				},
			},
		},
	}

	service := NewService(NewStore(nil, fallback))
	current, err := service.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if current.StoragePolicies.Policies[0].LocalPath != "data/uploads" {
		t.Fatalf("expected fallback local path, got %q", current.StoragePolicies.Policies[0].LocalPath)
	}
}

func TestUpdateStoragePoliciesRejectsInvalidDriver(t *testing.T) {
	service := NewService(NewStore(nil, RuntimeSettings{}))

	_, err := service.UpdateStoragePolicies(context.Background(), StoragePoliciesSettings{
		ActivePolicyID: "bad",
		Policies: []StoragePolicy{
			{ID: "bad", Name: "Bad", Driver: "ftp"},
		},
	})
	if err == nil {
		t.Fatal("expected invalid driver error")
	}
}

func TestUpdateStoragePoliciesAllowsLocalizedPolicyID(t *testing.T) {
	service := NewService(NewStore(nil, RuntimeSettings{}))

	settings, err := service.UpdateStoragePolicies(context.Background(), StoragePoliciesSettings{
		ActivePolicyID: "默认本地存储",
		Policies: []StoragePolicy{
			{ID: "默认本地存储", Name: "默认本地存储", Driver: "local", LocalPath: "data/uploads"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateStoragePolicies() returned error: %v", err)
	}
	if settings.ActivePolicyID != "默认本地存储" {
		t.Fatalf("expected localized active policy id, got %q", settings.ActivePolicyID)
	}
}

func TestUpdateStoragePoliciesGeneratesMissingPolicyID(t *testing.T) {
	service := NewService(NewStore(nil, RuntimeSettings{}))

	settings, err := service.UpdateStoragePolicies(context.Background(), StoragePoliciesSettings{
		Policies: []StoragePolicy{
			{ID: "  ", Name: "默认本地存储", Driver: "local", LocalPath: "data/uploads"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateStoragePolicies() returned error: %v", err)
	}
	if settings.Policies[0].ID != "default-local" {
		t.Fatalf("expected generated default policy id, got %q", settings.Policies[0].ID)
	}
	if settings.ActivePolicyID != "default-local" {
		t.Fatalf("expected generated active policy id, got %q", settings.ActivePolicyID)
	}
}

func TestUpdateStoragePoliciesKeepsUnchangedSecretKey(t *testing.T) {
	for _, incomingSecret := range []string{"", MaskedSecret} {
		t.Run("incoming="+incomingSecret, func(t *testing.T) {
			service := NewService(NewStore(nil, RuntimeSettings{
				StoragePolicies: StoragePoliciesSettings{
					ActivePolicyID: "minio",
					Policies: []StoragePolicy{
						{
							ID:     "minio",
							Name:   "MinIO",
							Driver: "minio",
							S3: S3Settings{
								Endpoint:  "http://127.0.0.1:9000",
								Bucket:    "fluffcatch",
								Region:    "us-east-1",
								AccessKey: "access",
								SecretKey: "persisted-secret",
							},
						},
					},
				},
			}))

			updated, err := service.UpdateStoragePolicies(context.Background(), StoragePoliciesSettings{
				ActivePolicyID: "minio",
				Policies: []StoragePolicy{
					{
						ID:     "minio",
						Name:   "MinIO updated",
						Driver: "minio",
						S3: S3Settings{
							Endpoint:  "http://127.0.0.1:9000",
							Bucket:    "fluffcatch",
							Region:    "us-east-1",
							AccessKey: "access",
							SecretKey: incomingSecret,
						},
					},
				},
			})
			if err != nil {
				t.Fatalf("UpdateStoragePolicies() returned error: %v", err)
			}
			if got := updated.Policies[0].S3.SecretKey; got != "persisted-secret" {
				t.Fatalf("expected persisted secret to be preserved, got %q", got)
			}
		})
	}
}

func TestUpdateSiteAllowsEmptyHomeMarkdown(t *testing.T) {
	service := NewService(NewStore(nil, RuntimeSettings{}))

	site, err := service.UpdateSite(context.Background(), SiteSettings{
		Name:               "Test",
		ThemeMode:          "  dark  ",
		ThemePreset:        "custom",
		ThemePrimaryColor:  "  #ABCDEF  ",
		ContactWidgetTitle: "  联系方式  ",
		ContactWidgetHTML:  "  <p>QQ 123456</p>  ",
		FooterSections: []FooterSection{
			{Title: "  关于站点  ", HTML: "  <p>Copyright 2026</p>  "},
			{Title: "  快速入口  ", HTML: `  <ul><li><a href="/">首页</a></li></ul>  `},
			{Title: "  站点信息  ", HTML: "  <p>粤ICP备12345678号</p>  "},
		},
	})
	if err != nil {
		t.Fatalf("UpdateSite() returned error: %v", err)
	}
	if site.HomeMarkdown != "" {
		t.Fatalf("expected empty home markdown, got %q", site.HomeMarkdown)
	}
	if len(site.FooterSections) != 3 {
		t.Fatalf("expected three footer sections, got %d", len(site.FooterSections))
	}
	if site.FooterSections[0].Title != "关于站点" || site.FooterSections[0].HTML != "<p>Copyright 2026</p>" {
		t.Fatalf("expected trimmed first footer section, got %#v", site.FooterSections[0])
	}
	if site.ContactWidgetTitle != "联系方式" || site.ContactWidgetHTML != "<p>QQ 123456</p>" {
		t.Fatalf("expected trimmed contact widget, got title=%q html=%q", site.ContactWidgetTitle, site.ContactWidgetHTML)
	}
	if site.ThemeMode != "dark" {
		t.Fatalf("expected normalized theme mode, got %q", site.ThemeMode)
	}
	if site.ThemePreset != "custom" {
		t.Fatalf("expected custom theme preset, got %q", site.ThemePreset)
	}
	if site.ThemePrimaryColor != "#abcdef" {
		t.Fatalf("expected normalized theme color, got %q", site.ThemePrimaryColor)
	}
}

func TestUpdateUploadNormalizesLimits(t *testing.T) {
	service := NewService(NewStore(nil, RuntimeSettings{}))

	upload, err := service.UpdateUpload(context.Background(), UploadSettings{})
	if err != nil {
		t.Fatalf("UpdateUpload() returned error: %v", err)
	}
	if upload.MaxFileSizeMB != 20 {
		t.Fatalf("expected default max file size 20, got %d", upload.MaxFileSizeMB)
	}
	if upload.MaxVideoSizeMB != 500 {
		t.Fatalf("expected default max video size 500, got %d", upload.MaxVideoSizeMB)
	}
	if upload.MaxFilesPerUpload != 20 {
		t.Fatalf("expected default max files 20, got %d", upload.MaxFilesPerUpload)
	}
	if upload.MaxConcurrentUploads != 2 {
		t.Fatalf("expected default concurrent uploads 2, got %d", upload.MaxConcurrentUploads)
	}

	upload, err = service.UpdateUpload(context.Background(), UploadSettings{MaxFileSizeMB: 2048, MaxVideoSizeMB: 20000, MaxFilesPerUpload: 500, MaxConcurrentUploads: 99})
	if err != nil {
		t.Fatalf("UpdateUpload() returned error for high limits: %v", err)
	}
	if upload.MaxFileSizeMB != 1024 {
		t.Fatalf("expected capped max file size 1024, got %d", upload.MaxFileSizeMB)
	}
	if upload.MaxFilesPerUpload != 200 {
		t.Fatalf("expected capped max files 200, got %d", upload.MaxFilesPerUpload)
	}
	if upload.MaxVideoSizeMB != 10240 {
		t.Fatalf("expected capped max video size 10240, got %d", upload.MaxVideoSizeMB)
	}
	if upload.MaxConcurrentUploads != 8 {
		t.Fatalf("expected capped concurrent uploads 8, got %d", upload.MaxConcurrentUploads)
	}
}

func TestUpdateStoragePoliciesPreventsDeletingUsedPolicy(t *testing.T) {
	service := NewServiceWithReferences(
		NewStore(nil, RuntimeSettings{
			StoragePolicies: StoragePoliciesSettings{
				ActivePolicyID: "old",
				Policies: []StoragePolicy{
					{ID: "old", Name: "Old", Driver: "local", LocalPath: "data/old", PublicPrefix: "/media"},
					{ID: "new", Name: "New", Driver: "local", LocalPath: "data/new", PublicPrefix: "/media"},
				},
			},
		}),
		fakeReferenceChecker{usage: map[string]PolicyUsage{
			"old": {PolicyID: "old", ObjectCount: 1, SizeBytes: 1024},
		}},
	)

	_, err := service.UpdateStoragePolicies(context.Background(), StoragePoliciesSettings{
		ActivePolicyID: "new",
		Policies: []StoragePolicy{
			{ID: "new", Name: "New", Driver: "local", LocalPath: "data/new", PublicPrefix: "/media"},
		},
	})
	if err == nil {
		t.Fatal("expected used policy deletion error")
	}
}

type fakeReferenceChecker struct {
	usage map[string]PolicyUsage
}

func (checker fakeReferenceChecker) Usage(ctx context.Context, policyIDs []string) (map[string]PolicyUsage, error) {
	_ = ctx
	result := map[string]PolicyUsage{}
	for _, policyID := range policyIDs {
		result[policyID] = checker.usage[policyID]
		result[policyID] = PolicyUsage{
			PolicyID:    policyID,
			ObjectCount: result[policyID].ObjectCount,
			SizeBytes:   result[policyID].SizeBytes,
		}
	}

	return result, nil
}
