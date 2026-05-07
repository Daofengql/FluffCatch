package settings

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

type Service struct {
	store      *Store
	references PolicyReferenceChecker
}

func NewService(store *Store) *Service {
	return &Service{
		store:      store,
		references: NoopPolicyReferenceChecker{},
	}
}

func NewServiceWithReferences(store *Store, references PolicyReferenceChecker) *Service {
	if references == nil {
		references = NoopPolicyReferenceChecker{}
	}

	return &Service{
		store:      store,
		references: references,
	}
}

func (service *Service) Load(ctx context.Context) (RuntimeSettings, error) {
	return service.store.Load(ctx)
}

func (service *Service) UpdateStoragePolicies(ctx context.Context, policies StoragePoliciesSettings) (StoragePoliciesSettings, error) {
	current, err := service.store.Load(ctx)
	if err != nil {
		return StoragePoliciesSettings{}, err
	}

	policies = preserveMaskedStorageSecrets(policies, current.StoragePolicies)
	normalized, err := normalizeStoragePolicies(policies)
	if err != nil {
		return StoragePoliciesSettings{}, err
	}

	removedIDs := removedPolicyIDs(current.StoragePolicies.Policies, normalized.Policies)
	if len(removedIDs) > 0 {
		usage, err := service.references.Usage(ctx, removedIDs)
		if err != nil {
			return StoragePoliciesSettings{}, err
		}
		for _, removedID := range removedIDs {
			if usage[removedID].ObjectCount > 0 {
				return StoragePoliciesSettings{}, fmt.Errorf("storage policy %q is still used by %d objects and cannot be deleted", removedID, usage[removedID].ObjectCount)
			}
		}
	}

	if err := service.store.SaveStoragePolicies(ctx, normalized); err != nil {
		return StoragePoliciesSettings{}, err
	}

	return normalized, nil
}

func (service *Service) StoragePolicyUsage(ctx context.Context) (map[string]PolicyUsage, error) {
	current, err := service.store.Load(ctx)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(current.StoragePolicies.Policies))
	for _, policy := range current.StoragePolicies.Policies {
		ids = append(ids, policy.ID)
	}

	return service.references.Usage(ctx, ids)
}

type PolicyUsage struct {
	PolicyID    string `json:"policyId"`
	ObjectCount int64  `json:"objectCount"`
	SizeBytes   int64  `json:"sizeBytes"`
}

type PolicyReferenceChecker interface {
	Usage(ctx context.Context, policyIDs []string) (map[string]PolicyUsage, error)
}

type NoopPolicyReferenceChecker struct{}

func (NoopPolicyReferenceChecker) Usage(ctx context.Context, policyIDs []string) (map[string]PolicyUsage, error) {
	_ = ctx
	usage := make(map[string]PolicyUsage, len(policyIDs))
	for _, policyID := range policyIDs {
		usage[policyID] = PolicyUsage{PolicyID: policyID}
	}

	return usage, nil
}

type SQLPolicyReferenceChecker struct {
	db *sql.DB
}

func NewSQLPolicyReferenceChecker(dbConn *sql.DB) PolicyReferenceChecker {
	if dbConn == nil {
		return NoopPolicyReferenceChecker{}
	}

	return SQLPolicyReferenceChecker{db: dbConn}
}

func (checker SQLPolicyReferenceChecker) Usage(ctx context.Context, policyIDs []string) (map[string]PolicyUsage, error) {
	usage := make(map[string]PolicyUsage, len(policyIDs))
	for _, policyID := range policyIDs {
		usage[policyID] = PolicyUsage{PolicyID: policyID}
	}

	if len(policyIDs) == 0 {
		return usage, nil
	}

	placeholders := make([]string, len(policyIDs))
	args := make([]any, len(policyIDs))
	for index, policyID := range policyIDs {
		placeholders[index] = "?"
		args[index] = policyID
	}

	query := fmt.Sprintf(`
		SELECT storage_policy_id, SUM(object_count) AS object_count, SUM(size_bytes) AS size_bytes
		FROM (
			SELECT storage_policy_id, COUNT(*) AS object_count, COALESCE(SUM(size_bytes), 0) AS size_bytes
			FROM photos
			WHERE storage_policy_id IN (%[1]s)
			GROUP BY storage_policy_id
			UNION ALL
			SELECT storage_policy_id, COUNT(*) AS object_count, COALESCE(SUM(size_bytes), 0) AS size_bytes
			FROM submissions
			WHERE storage_policy_id IN (%[1]s)
			GROUP BY storage_policy_id
			UNION ALL
			SELECT cover_storage_policy_id AS storage_policy_id, COUNT(*) AS object_count, 0 AS size_bytes
			FROM events
			WHERE cover_storage_policy_id IN (%[1]s)
			GROUP BY cover_storage_policy_id
		) AS policy_usage
		GROUP BY storage_policy_id
	`, strings.Join(placeholders, ","))

	allArgs := append([]any{}, args...)
	allArgs = append(allArgs, args...)
	allArgs = append(allArgs, args...)

	rows, err := checker.db.QueryContext(ctx, query, allArgs...)
	if err != nil {
		return nil, fmt.Errorf("load storage policy usage: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item PolicyUsage
		if err := rows.Scan(&item.PolicyID, &item.ObjectCount, &item.SizeBytes); err != nil {
			return nil, fmt.Errorf("scan storage policy usage: %w", err)
		}
		usage[item.PolicyID] = item
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate storage policy usage: %w", err)
	}

	return usage, nil
}

func removedPolicyIDs(before []StoragePolicy, after []StoragePolicy) []string {
	afterIDs := map[string]struct{}{}
	for _, policy := range after {
		afterIDs[policy.ID] = struct{}{}
	}

	var removed []string
	for _, policy := range before {
		if _, exists := afterIDs[policy.ID]; !exists {
			removed = append(removed, policy.ID)
		}
	}

	return removed
}

func (service *Service) UpdateOIDC(ctx context.Context, oidc OIDCSettings) (OIDCSettings, error) {
	current, err := service.store.Load(ctx)
	if err == nil {
		oidc = preserveMaskedOIDCSecret(oidc, current.OIDC)
	}
	normalized := normalizeOIDC(oidc)
	if err := service.store.SaveOIDC(ctx, normalized); err != nil {
		return OIDCSettings{}, err
	}

	return normalized, nil
}

func (service *Service) UpdateSite(ctx context.Context, site SiteSettings) (SiteSettings, error) {
	normalized := normalizeSite(site)
	if err := service.store.SaveSite(ctx, normalized); err != nil {
		return SiteSettings{}, err
	}
	return normalized, nil
}

func normalizeStoragePolicies(policies StoragePoliciesSettings) (StoragePoliciesSettings, error) {
	if len(policies.Policies) == 0 {
		return StoragePoliciesSettings{}, fmt.Errorf("at least one storage policy is required")
	}

	policies.ActivePolicyID = strings.TrimSpace(policies.ActivePolicyID)
	seen := map[string]struct{}{}
	activeFound := false

	for index, policy := range policies.Policies {
		normalized, err := NormalizeStoragePolicy(policy)
		if err != nil {
			return StoragePoliciesSettings{}, err
		}

		if _, exists := seen[normalized.ID]; exists {
			return StoragePoliciesSettings{}, fmt.Errorf("duplicate storage policy id %q", normalized.ID)
		}
		seen[normalized.ID] = struct{}{}

		if normalized.ID == policies.ActivePolicyID {
			activeFound = true
		}

		policies.Policies[index] = normalized
	}

	if policies.ActivePolicyID == "" {
		policies.ActivePolicyID = policies.Policies[0].ID
		activeFound = true
	}

	if !activeFound {
		return StoragePoliciesSettings{}, fmt.Errorf("active storage policy %q does not exist", policies.ActivePolicyID)
	}

	return policies, nil
}

func NormalizeStoragePolicy(policy StoragePolicy) (StoragePolicy, error) {
	policy.ID = strings.TrimSpace(policy.ID)
	if !validPolicyID(policy.ID) {
		return StoragePolicy{}, fmt.Errorf("storage policy id must use letters, numbers, dash or underscore")
	}

	policy.Name = strings.TrimSpace(policy.Name)
	if policy.Name == "" {
		return StoragePolicy{}, fmt.Errorf("storage policy name is required")
	}

	policy.Driver = strings.ToLower(strings.TrimSpace(policy.Driver))
	if policy.Driver == "" {
		policy.Driver = "local"
	}

	switch policy.Driver {
	case "local":
		if strings.TrimSpace(policy.LocalPath) == "" {
			return StoragePolicy{}, fmt.Errorf("localPath is required for local storage")
		}
	case "aws-s3":
		if strings.TrimSpace(policy.S3.Bucket) == "" {
			return StoragePolicy{}, fmt.Errorf("bucket is required for AWS S3")
		}
		if strings.TrimSpace(policy.S3.AccessKey) == "" || strings.TrimSpace(policy.S3.SecretKey) == "" {
			return StoragePolicy{}, fmt.Errorf("accessKey and secretKey are required for AWS S3")
		}
		if strings.TrimSpace(policy.S3.Region) == "" {
			policy.S3.Region = "us-east-1"
		}
		if strings.TrimSpace(policy.S3.Endpoint) == "" {
			policy.S3.Endpoint = "s3.amazonaws.com"
		}
		policy.S3.UseSSL = true
	case "minio":
		if strings.TrimSpace(policy.S3.Endpoint) == "" {
			return StoragePolicy{}, fmt.Errorf("endpoint is required for MinIO")
		}
		if strings.TrimSpace(policy.S3.Bucket) == "" {
			return StoragePolicy{}, fmt.Errorf("bucket is required for MinIO")
		}
		if strings.TrimSpace(policy.S3.AccessKey) == "" || strings.TrimSpace(policy.S3.SecretKey) == "" {
			return StoragePolicy{}, fmt.Errorf("accessKey and secretKey are required for MinIO")
		}
	case "aliyun-oss":
		if strings.TrimSpace(policy.S3.Endpoint) == "" {
			return StoragePolicy{}, fmt.Errorf("endpoint is required for Aliyun OSS")
		}
		if strings.TrimSpace(policy.S3.Bucket) == "" {
			return StoragePolicy{}, fmt.Errorf("bucket is required for Aliyun OSS")
		}
		if strings.TrimSpace(policy.S3.AccessKey) == "" || strings.TrimSpace(policy.S3.SecretKey) == "" {
			return StoragePolicy{}, fmt.Errorf("accessKey and secretKey are required for Aliyun OSS")
		}
	case "tencent-cos":
		if strings.TrimSpace(policy.S3.Endpoint) == "" {
			return StoragePolicy{}, fmt.Errorf("endpoint is required for Tencent COS")
		}
		if strings.TrimSpace(policy.S3.Bucket) == "" {
			return StoragePolicy{}, fmt.Errorf("bucket is required for Tencent COS")
		}
		if strings.TrimSpace(policy.S3.AccessKey) == "" || strings.TrimSpace(policy.S3.SecretKey) == "" {
			return StoragePolicy{}, fmt.Errorf("accessKey and secretKey are required for Tencent COS")
		}
	case "cf-r2":
		if strings.TrimSpace(policy.S3.AccountID) == "" {
			return StoragePolicy{}, fmt.Errorf("accountId is required for Cloudflare R2")
		}
		if strings.TrimSpace(policy.S3.Bucket) == "" {
			return StoragePolicy{}, fmt.Errorf("bucket is required for Cloudflare R2")
		}
		if strings.TrimSpace(policy.S3.AccessKey) == "" || strings.TrimSpace(policy.S3.SecretKey) == "" {
			return StoragePolicy{}, fmt.Errorf("accessKey and secretKey are required for Cloudflare R2")
		}
		policy.S3.Endpoint = fmt.Sprintf("%s.r2.cloudflarestorage.com", strings.TrimSpace(policy.S3.AccountID))
		policy.S3.UseSSL = true
		if strings.TrimSpace(policy.S3.Region) == "" {
			policy.S3.Region = "auto"
		}
	case "s3":
		if strings.TrimSpace(policy.S3.Endpoint) == "" {
			return StoragePolicy{}, fmt.Errorf("endpoint is required for S3 storage")
		}
		if strings.TrimSpace(policy.S3.Bucket) == "" {
			return StoragePolicy{}, fmt.Errorf("bucket is required for S3 storage")
		}
	default:
		return StoragePolicy{}, fmt.Errorf("unsupported storage driver %q", policy.Driver)
	}

	if strings.TrimSpace(policy.PublicPrefix) == "" {
		policy.PublicPrefix = "/media"
	}
	policy.PublicBaseURL = strings.TrimRight(strings.TrimSpace(policy.PublicBaseURL), "/")
	if strings.TrimSpace(policy.S3.Region) == "" {
		policy.S3.Region = "us-east-1"
	}

	return policy, nil
}

func normalizeOIDC(oidc OIDCSettings) OIDCSettings {
	oidc.Provider = strings.TrimSpace(oidc.Provider)
	oidc.IssuerURL = strings.TrimSpace(oidc.IssuerURL)
	oidc.ClientID = strings.TrimSpace(oidc.ClientID)
	oidc.RedirectURL = strings.TrimSpace(oidc.RedirectURL)
	return oidc
}

func normalizeSite(site SiteSettings) SiteSettings {
	site.Name = strings.TrimSpace(site.Name)
	if site.Name == "" {
		site.Name = "FluffCatch"
	}
	site.Subtitle = strings.TrimSpace(site.Subtitle)
	site.LogoURL = strings.TrimSpace(site.LogoURL)
	site.HomeMarkdown = strings.TrimSpace(site.HomeMarkdown)
	return site
}

var policyIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func validPolicyID(id string) bool {
	return id != "" && policyIDPattern.MatchString(id)
}

func preserveMaskedStorageSecrets(incoming, current StoragePoliciesSettings) StoragePoliciesSettings {
	currentByID := make(map[string]StoragePolicy, len(current.Policies))
	for _, p := range current.Policies {
		currentByID[p.ID] = p
	}
	for i, p := range incoming.Policies {
		if p.S3.SecretKey != MaskedSecret {
			continue
		}
		if cur, ok := currentByID[p.ID]; ok {
			incoming.Policies[i].S3.SecretKey = cur.S3.SecretKey
		}
	}
	return incoming
}

func preserveMaskedOIDCSecret(incoming, current OIDCSettings) OIDCSettings {
	if incoming.ClientSecret == MaskedSecret {
		incoming.ClientSecret = current.ClientSecret
	}
	return incoming
}
