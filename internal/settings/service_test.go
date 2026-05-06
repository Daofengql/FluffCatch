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
