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

func TestUpdateSiteAllowsEmptyHomeMarkdown(t *testing.T) {
	service := NewService(NewStore(nil, RuntimeSettings{}))

	site, err := service.UpdateSite(context.Background(), SiteSettings{
		Name:              "Test",
		ThemeMode:         "  dark  ",
		ThemePreset:       "custom",
		ThemePrimaryColor: "  #ABCDEF  ",
		FooterText:        "  Copyright 2026  ",
		ICPNumber:         "  粤ICP备12345678号  ",
		ContactEmail:      "  hello@example.com  ",
	})
	if err != nil {
		t.Fatalf("UpdateSite() returned error: %v", err)
	}
	if site.HomeMarkdown != "" {
		t.Fatalf("expected empty home markdown, got %q", site.HomeMarkdown)
	}
	if site.FooterText != "Copyright 2026" {
		t.Fatalf("expected trimmed footer text, got %q", site.FooterText)
	}
	if site.ICPNumber != "粤ICP备12345678号" {
		t.Fatalf("expected trimmed ICP number, got %q", site.ICPNumber)
	}
	if site.ContactEmail != "hello@example.com" {
		t.Fatalf("expected trimmed contact email, got %q", site.ContactEmail)
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
	if upload.MaxFilesPerUpload != 20 {
		t.Fatalf("expected default max files 20, got %d", upload.MaxFilesPerUpload)
	}

	upload, err = service.UpdateUpload(context.Background(), UploadSettings{MaxFileSizeMB: 2048, MaxFilesPerUpload: 500})
	if err != nil {
		t.Fatalf("UpdateUpload() returned error for high limits: %v", err)
	}
	if upload.MaxFileSizeMB != 1024 {
		t.Fatalf("expected capped max file size 1024, got %d", upload.MaxFileSizeMB)
	}
	if upload.MaxFilesPerUpload != 200 {
		t.Fatalf("expected capped max files 200, got %d", upload.MaxFilesPerUpload)
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
