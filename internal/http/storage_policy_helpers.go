package http

import (
	stdhttp "net/http"

	"fluffcatch/internal/settings"
	"fluffcatch/internal/storage"
)

func (server *Server) storagePolicyUsageOrEmpty(r *stdhttp.Request) map[string]settings.PolicyUsage {
	usage, err := server.settingsService.StoragePolicyUsageForIDs(r.Context(), storagePolicyIDs(server.runtimeStoragePolicies().Policies))
	if err != nil {
		return map[string]settings.PolicyUsage{}
	}

	return usage
}

func (server *Server) runtimeStoragePolicies() settings.StoragePoliciesSettings {
	configs := server.storageManager.Configs()
	policies := make([]settings.StoragePolicy, 0, len(configs))
	for _, config := range configs {
		policies = append(policies, storagePolicyFromConfig(config))
	}

	return settings.StoragePoliciesSettings{
		ActivePolicyID: server.storageManager.ActivePolicyID(),
		Policies:       policies,
	}
}

func storagePolicyIDs(policies []settings.StoragePolicy) []string {
	ids := make([]string, 0, len(policies))
	for _, policy := range policies {
		if policy.ID != "" {
			ids = append(ids, policy.ID)
		}
	}
	return ids
}

func storageConfigsFromPolicies(policies []settings.StoragePolicy) []storage.Config {
	configs := make([]storage.Config, 0, len(policies))
	for _, policy := range policies {
		configs = append(configs, storageConfigFromPolicy(policy))
	}

	return configs
}

func storageConfigFromPolicy(policy settings.StoragePolicy) storage.Config {
	return storage.Config{
		PolicyID:      policy.ID,
		Name:          policy.Name,
		Driver:        policy.Driver,
		LocalPath:     policy.LocalPath,
		PublicPrefix:  policy.PublicPrefix,
		PublicBaseURL: policy.PublicBaseURL,
		S3: storage.S3Config{
			Endpoint:  policy.S3.Endpoint,
			Bucket:    policy.S3.Bucket,
			Region:    policy.S3.Region,
			AccessKey: policy.S3.AccessKey,
			SecretKey: policy.S3.SecretKey,
			UseSSL:    policy.S3.UseSSL,
			AccountID: policy.S3.AccountID,
		},
	}
}

func storagePolicyFromConfig(config storage.Config) settings.StoragePolicy {
	return settings.StoragePolicy{
		ID:            config.PolicyID,
		Name:          config.Name,
		Driver:        config.Driver,
		LocalPath:     config.LocalPath,
		PublicPrefix:  config.PublicPrefix,
		PublicBaseURL: config.PublicBaseURL,
		S3: settings.S3Settings{
			Endpoint:  config.S3.Endpoint,
			Bucket:    config.S3.Bucket,
			Region:    config.S3.Region,
			AccessKey: config.S3.AccessKey,
			SecretKey: config.S3.SecretKey,
			UseSSL:    config.S3.UseSSL,
			AccountID: config.S3.AccountID,
		},
	}
}
