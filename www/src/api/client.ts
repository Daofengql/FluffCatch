export type EventCard = {
  id: number;
  title: string;
  description: string;
  location: string;
  provinceCode?: string;
  provinceName?: string;
  cityCode?: string;
  cityName?: string;
  startTime?: string;
  endTime?: string;
  coverPolicyId?: string;
  coverObjectKey?: string;
  coverUrl?: string;
  isPublic: boolean;
  submissionEnabled: boolean;
  privatePassword?: string;
  photoCount: number;
};

export type Photo = {
  id: number;
  eventId: number;
  storagePolicyId: string;
  objectKey: string;
  url: string;
  thumbnailUrl?: string;
  accessGranted: boolean;
  contentHash: string;
  contentType: string;
  sizeBytes: number;
  likeCount: number;
  liked: boolean;
  photographerName?: string;
  visibility: 'public' | 'private';
  tags: { id: number; name: string }[];
  exif?: Record<string, unknown>;
  takenAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type Submission = {
  id: number;
  eventId: number;
  storagePolicyId: string;
  objectKey: string;
  url: string;
  thumbnailUrl?: string;
  contentHash: string;
  contentType: string;
  sizeBytes: number;
  photographerName?: string;
  tags: string[];
  status: string;
  exif?: Record<string, unknown>;
  takenAt?: string;
  createdAt: string;
};

export type GalleryFilters = {
  tag?: string;
  photographer?: string;
  mediaType?: 'all' | 'image' | 'video';
  sort?: 'latest' | 'oldest' | 'taken_desc' | 'taken_asc' | 'likes';
};

export type SubmissionLink = {
  id: number;
  eventId: number;
  label: string;
  photographerName?: string;
  token?: string;
  expiresAt?: string;
  maxUses: number;
  useCount: number;
  revokedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type PhotoPage = {
  photos: Photo[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
};

export type EventPage = {
  events: EventCard[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
};

export type StoragePolicyUsage = {
  policyId: string;
  objectCount: number;
  sizeBytes: number;
};

export type S3Settings = {
  endpoint: string;
  bucket: string;
  region: string;
  accessKey: string;
  secretKey?: string;
  useSsl: boolean;
  accountId?: string;
};

export type StorageDriver = 'local' | 'minio' | 'aws-s3' | 'aliyun-oss' | 'tencent-cos' | 'cf-r2' | 's3';

export type StoragePolicy = {
  id: string;
  name: string;
  driver: StorageDriver;
  localPath?: string;
  publicPrefix: string;
  publicBaseUrl?: string;
  s3?: S3Settings;
};

export type FooterSection = {
  title: string;
  html: string;
};

export type SiteSettings = {
  name: string;
  subtitle: string;
  logoUrl: string;
  homeMarkdown: string;
  themeMode: 'system' | 'light' | 'dark';
  themePreset: 'blue' | 'emerald' | 'rose' | 'amber' | 'violet' | 'custom';
  themePrimaryColor: string;
  publicBackgroundDesktopUrl: string;
  publicBackgroundMobileUrl: string;
  footerSections: FooterSection[];
  contactWidgetEnabled: boolean;
  contactWidgetTitle: string;
  contactWidgetHtml: string;
};

export type UploadSettings = {
  maxFileSizeMb: number;
  maxVideoSizeMb: number;
  maxFilesPerUpload: number;
  defaultPageSize: number;
  maxConcurrentUploads: number;
};

export type PublicOIDCSettings = {
  enabled: boolean;
  providerName: string;
};

export type OIDCStatus = {
  enabled: boolean;
  bound: boolean;
  subject?: string;
  providerName?: string;
};

export type MeResponse = {
  authenticated: boolean;
  username?: string;
};

export type AdminSettingsResponse = {
  settings: {
    site: SiteSettings;
    upload: UploadSettings;
    storagePolicies: {
      activePolicyId: string;
      policies: StoragePolicy[];
    };
  };
  usage: Record<string, StoragePolicyUsage>;
};

type CacheOptions = {
  dedupe?: boolean;
  tags?: string[];
  ttlMs?: number;
};

type CachedResponse = {
  expiresAt: number;
  tags: Set<string>;
  value: unknown;
};

const SITE_CACHE_TAG = 'site';
const EVENTS_CACHE_TAG = 'events';
const AUTH_CACHE_TAG = 'auth';
const ADMIN_SETTINGS_CACHE_TAG = 'admin-settings';
const ADMIN_DASHBOARD_CACHE_TAG = 'admin-dashboard';
const DEFAULT_DEDUPE_TTL_MS = 1500;
const getResponseCache = new Map<string, CachedResponse>();
const pendingGetRequests = new Map<string, Promise<unknown>>();

function cacheKey(url: string) {
  return `GET ${url}`;
}

function invalidateCacheTags(...tags: string[]) {
  if (!tags.length) return;
  for (const [key, entry] of getResponseCache) {
    if (tags.some((tag) => entry.tags.has(tag))) {
      getResponseCache.delete(key);
    }
  }
}

async function request<T>(url: string, options?: RequestInit, cache?: CacheOptions): Promise<T> {
  const method = (options?.method || 'GET').toUpperCase();
  const canCache = method === 'GET' && !options?.body && Boolean(cache?.dedupe || cache?.ttlMs);
  const key = canCache ? cacheKey(url) : '';

  if (canCache) {
    const cached = getResponseCache.get(key);
    if (cached && cached.expiresAt > Date.now()) {
      return cached.value as T;
    }
    const pending = pendingGetRequests.get(key);
    if (pending) {
      return pending as Promise<T>;
    }
  }

  const promise = fetch(url, {
    credentials: 'include',
    ...options,
    headers: {
      ...(options?.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
      ...options?.headers
    }
  })
    .then(async (response) => {
      if (!response.ok) {
        const payload = await response.json().catch(() => ({ error: response.statusText }));
        const message = payload.error || payload.message || response.statusText || '请求失败';
        throw new Error(message);
      }
      return (await response.json()) as T;
    })
    .then((payload) => {
      if (canCache) {
        getResponseCache.set(key, {
          expiresAt: Date.now() + (cache?.ttlMs ?? DEFAULT_DEDUPE_TTL_MS),
          tags: new Set(cache?.tags ?? []),
          value: payload
        });
      }
      return payload;
    })
    .finally(() => {
      if (canCache) {
        pendingGetRequests.delete(key);
      }
    });

  if (canCache) {
    pendingGetRequests.set(key, promise);
  }

  return promise;
}

export type CaptchaChallenge = {
  id: string;
  imageSvg: string;
  expiresAt: string;
};

export function captchaHeaders(challenge: CaptchaChallenge, answer: string): Record<string, string> {
  return { 'X-Captcha-Id': challenge.id, 'X-Captcha-Answer': answer };
}

export async function getCaptcha() {
  return request<CaptchaChallenge>('/api/v1/auth/captcha');
}

export async function getSiteSettings(): Promise<SiteSettings> {
  return request<SiteSettings>('/api/v1/site', undefined, {
    dedupe: true,
    tags: [SITE_CACHE_TAG],
    ttlMs: 30_000
  });
}

export async function getUploadSettings(): Promise<UploadSettings> {
  return request<UploadSettings>('/api/v1/settings/upload', undefined, {
    dedupe: true,
    tags: [SITE_CACHE_TAG],
    ttlMs: 30_000
  });
}

export async function login(username: string, password: string, captchaId: string, captchaAnswer: string) {
  const result = await request<{ authenticated: boolean; username?: string; message: string }>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password, captchaId, captchaAnswer })
  });
  invalidateCacheTags(AUTH_CACHE_TAG, EVENTS_CACHE_TAG, ADMIN_SETTINGS_CACHE_TAG, ADMIN_DASHBOARD_CACHE_TAG);
  return result;
}

export async function logout() {
  const result = await request<{ message: string }>('/api/v1/auth/logout', { method: 'POST' });
  invalidateCacheTags(AUTH_CACHE_TAG, EVENTS_CACHE_TAG, ADMIN_SETTINGS_CACHE_TAG, ADMIN_DASHBOARD_CACHE_TAG);
  return result;
}

export async function getMe() {
  return request<MeResponse>('/api/v1/auth/me', undefined, {
    dedupe: true,
    tags: [AUTH_CACHE_TAG],
    ttlMs: 2_000
  });
}

export async function getPublicOIDCSettings() {
  return request<PublicOIDCSettings>('/api/v1/auth/oidc', undefined, {
    dedupe: true,
    tags: [SITE_CACHE_TAG],
    ttlMs: 30_000
  });
}

export async function getOIDCLoginURL() {
  return request<{ url: string }>('/api/v1/auth/oidc/login');
}

export type EventListFilters = {
  cityCode?: string;
  endDate?: string;
  page?: number;
  pageSize?: number;
  provinceCode?: string;
  query?: string;
  sort?: 'start_desc' | 'start_asc';
  startDate?: string;
};

export async function getEventPage(filters: EventListFilters = {}): Promise<EventPage> {
  const params = new URLSearchParams();
  if (filters.query?.trim()) {
    params.set('q', filters.query.trim());
  }
  if (filters.provinceCode?.trim()) params.set('provinceCode', filters.provinceCode.trim());
  if (filters.cityCode?.trim()) params.set('cityCode', filters.cityCode.trim());
  if (filters.startDate?.trim()) params.set('startDate', filters.startDate.trim());
  if (filters.endDate?.trim()) params.set('endDate', filters.endDate.trim());
  if (filters.sort && filters.sort !== 'start_desc') params.set('sort', filters.sort);
  if (filters.page && filters.page > 1) params.set('page', String(filters.page));
  if (filters.pageSize && filters.pageSize > 0) params.set('pageSize', String(filters.pageSize));
  const query = params.toString();
  const payload = await request<EventPage | null>(`/api/v1/events${query ? `?${query}` : ''}`, undefined, {
    dedupe: true,
    tags: [EVENTS_CACHE_TAG],
    ttlMs: 3_000
  });
  return {
    events: Array.isArray(payload?.events) ? payload.events : [],
    total: payload?.total ?? 0,
    page: payload?.page ?? filters.page ?? 1,
    pageSize: payload?.pageSize ?? filters.pageSize ?? 24,
    totalPages: payload?.totalPages ?? 0
  };
}

export async function getEvents(admin = false, filters: { query?: string } = {}): Promise<EventCard[]> {
  if (!admin) {
    const page = await getEventPage({ query: filters.query });
    return page.events;
  }
  const payload = await request<{ events?: EventCard[] } | null>('/api/v1/admin/events', undefined, {
    dedupe: true,
    tags: [EVENTS_CACHE_TAG],
    ttlMs: 3_000
  });
  return Array.isArray(payload?.events) ? payload.events : [];
}

export async function getEvent(id: number): Promise<EventCard> {
  const payload = await request<{ event: EventCard }>(`/api/v1/events/${id}`);
  return payload.event;
}

export async function getPhotos(eventId: number, admin = false, page = 1, pageSize?: number, visibility: 'all' | Photo['visibility'] = 'all', filters: GalleryFilters = {}): Promise<PhotoPage> {
  const query = new URLSearchParams({ page: String(page) });
  if (pageSize && pageSize > 0) {
    query.set('pageSize', String(pageSize));
  }
  if (visibility !== 'all') {
    query.set('visibility', visibility);
  }
  if (filters.tag?.trim()) query.set('tag', filters.tag.trim());
  if (filters.photographer?.trim()) query.set('photographer', filters.photographer.trim());
  if (filters.mediaType && filters.mediaType !== 'all') query.set('mediaType', filters.mediaType);
  if (filters.sort && filters.sort !== 'latest') query.set('sort', filters.sort);
  const payload = await request<PhotoPage>(admin ? `/api/v1/admin/events/${eventId}/photos?${query}` : `/api/v1/events/${eventId}/photos?${query}`);
  return {
    photos: payload.photos ?? [],
    total: payload.total ?? 0,
    page: payload.page ?? page,
    pageSize: payload.pageSize ?? pageSize ?? 24,
    totalPages: payload.totalPages ?? 0
  };
}

export async function unlockEventPrivatePhotos(eventId: number, password: string) {
  return request<{ unlocked: boolean }>(`/api/v1/events/${eventId}/private-access`, {
    method: 'POST',
    body: JSON.stringify({ password })
  });
}

export async function getPhotoList(eventId: number, admin = false): Promise<Photo[]> {
  const payload = await getPhotos(eventId, admin, 1, 100);
  return payload.photos ?? [];
}

export async function saveEvent(event: Partial<EventCard> & { clearPrivatePassword?: boolean; privatePassword?: string }) {
  const body = JSON.stringify({
    title: event.title,
    description: event.description,
    location: event.location,
    provinceCode: event.provinceCode || '',
    provinceName: event.provinceName || '',
    cityCode: event.cityCode || '',
    cityName: event.cityName || '',
    startTime: event.startTime || '',
    endTime: event.endTime || '',
    coverPolicyId: event.coverPolicyId || '',
    coverObjectKey: event.coverObjectKey || '',
    removeCover: false,
    isPublic: Boolean(event.isPublic),
    submissionEnabled: event.submissionEnabled ?? true,
    privatePassword: event.privatePassword || '',
    clearPrivatePassword: Boolean(event.clearPrivatePassword)
  });
  if (event.id) {
    const result = await request<{ event: EventCard }>(`/api/v1/admin/events/${event.id}`, { method: 'PUT', body });
    invalidateCacheTags(EVENTS_CACHE_TAG, ADMIN_DASHBOARD_CACHE_TAG);
    return result;
  }
  const result = await request<{ event: EventCard }>('/api/v1/admin/events', { method: 'POST', body });
  invalidateCacheTags(EVENTS_CACHE_TAG, ADMIN_DASHBOARD_CACHE_TAG);
  return result;
}

export async function uploadEventCover(eventId: number, file: File) {
  const form = new FormData();
  form.append('file', file);
  const result = await request<{ policyId: string; objectKey: string; url: string }>(`/api/v1/admin/events/${eventId}/cover`, {
    method: 'POST',
    body: form
  });
  invalidateCacheTags(EVENTS_CACHE_TAG);
  return result;
}

export async function setEventCoverFromPhoto(eventId: number, photoId: number) {
  const result = await request<{ policyId: string; objectKey: string; url: string }>(`/api/v1/admin/events/${eventId}/cover-from-photo`, {
    method: 'POST',
    body: JSON.stringify({ photoId })
  });
  invalidateCacheTags(EVENTS_CACHE_TAG);
  return result;
}

export async function deleteEvent(eventId: number, headers?: Record<string, string>) {
  const result = await request<{ message: string; deletedObjects: number }>(`/api/v1/admin/events/${eventId}`, {
    method: 'DELETE',
    headers
  });
  invalidateCacheTags(EVENTS_CACHE_TAG, ADMIN_DASHBOARD_CACHE_TAG);
  return result;
}

export async function updatePhoto(photoId: number, payload: { photographerName?: string; visibility: Photo['visibility']; tags?: string[]; takenAt?: string }) {
  const result = await request<{ photo: Photo }>(`/api/v1/admin/photos/${photoId}`, {
    method: 'PUT',
    body: JSON.stringify(payload)
  });
  invalidateCacheTags(EVENTS_CACHE_TAG, ADMIN_DASHBOARD_CACHE_TAG);
  return result;
}

export async function deletePhoto(photoId: number, headers?: Record<string, string>) {
  const result = await request<{ message: string; deletedObjects: number }>(`/api/v1/admin/photos/${photoId}`, {
    method: 'DELETE',
    headers
  });
  invalidateCacheTags(EVENTS_CACHE_TAG, ADMIN_DASHBOARD_CACHE_TAG);
  return result;
}

export async function batchDeletePhotos(photoIds: number[], headers?: Record<string, string>) {
  const result = await request<{ deleted: number; deletedObjects: number }>('/api/v1/admin/photos/batch-delete', {
    method: 'POST',
    body: JSON.stringify({ photoIds }),
    headers
  });
  invalidateCacheTags(EVENTS_CACHE_TAG, ADMIN_DASHBOARD_CACHE_TAG);
  return result;
}

export async function batchUpdatePhotos(photoIds: number[], visibility?: Photo['visibility'], payload: { photographerName?: string; tags?: string[]; replaceTags?: boolean } = {}) {
  const result = await request<{ affected: number; message: string }>('/api/v1/admin/photos/batch-update', {
    method: 'POST',
    body: JSON.stringify({ photoIds, visibility, photographerName: payload.photographerName, tags: payload.tags, replaceTags: payload.replaceTags })
  });
  invalidateCacheTags(EVENTS_CACHE_TAG);
  return result;
}

export async function likePhoto(photoId: number) {
  return request<{ photoId: number; likeCount: number; liked: boolean; justLiked: boolean }>(`/api/v1/photos/${photoId}/like`, {
    method: 'POST'
  });
}

export type SubmissionUploadResult = {
  submissions?: Submission[];
  photos?: Photo[];
};

export async function submitPhotos(eventId: number, form: FormData) {
  const result = await request<SubmissionUploadResult>(`/api/v1/events/${eventId}/submissions`, {
    method: 'POST',
    body: form
  });
  invalidateCacheTags(EVENTS_CACHE_TAG, ADMIN_DASHBOARD_CACHE_TAG);
  return result;
}

export async function resolveSubmissionToken(eventId: number, token: string) {
  const query = new URLSearchParams({ token });
  return request<{ valid: boolean; link?: SubmissionLink }>(`/api/v1/events/${eventId}/submission-token?${query}`);
}

export function submitPhotoWithProgress(eventId: number, form: FormData, onProgress: (progress: number) => void, signal?: AbortSignal) {
  return new Promise<SubmissionUploadResult>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const handleAbort = () => xhr.abort();
    if (signal?.aborted) {
      reject(new DOMException('上传已取消', 'AbortError'));
      return;
    }
    xhr.open('POST', `/api/v1/events/${eventId}/submissions`);
    xhr.withCredentials = true;
    signal?.addEventListener('abort', handleAbort, { once: true });
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable && event.total > 0) {
        onProgress(Math.max(1, Math.min(99, Math.round((event.loaded / event.total) * 100))));
      }
    };
    xhr.onload = () => {
      signal?.removeEventListener('abort', handleAbort);
      const payload = JSON.parse(xhr.responseText || '{}');
      if (xhr.status >= 200 && xhr.status < 300) {
        onProgress(100);
        invalidateCacheTags(EVENTS_CACHE_TAG, ADMIN_DASHBOARD_CACHE_TAG);
        resolve(payload);
        return;
      }
      reject(new Error(payload.error || xhr.statusText || '上传失败'));
    };
    xhr.onerror = () => {
      signal?.removeEventListener('abort', handleAbort);
      reject(new Error('网络错误，上传失败'));
    };
    xhr.onabort = () => {
      signal?.removeEventListener('abort', handleAbort);
      reject(new DOMException('上传已取消', 'AbortError'));
    };
    xhr.send(form);
  });
}

export async function getPendingSubmissions(): Promise<Submission[]> {
  const payload = await request<{ submissions?: Submission[] } | null>('/api/v1/admin/submissions');
  return Array.isArray(payload?.submissions) ? payload.submissions : [];
}

export async function getEventPendingSubmissions(eventId: number): Promise<Submission[]> {
  const payload = await request<{ submissions?: Submission[] } | null>(`/api/v1/admin/events/${eventId}/submissions`);
  return Array.isArray(payload?.submissions) ? payload.submissions : [];
}

export async function getSubmissionLinks(eventId: number): Promise<SubmissionLink[]> {
  const payload = await request<{ links?: SubmissionLink[] } | null>(`/api/v1/admin/events/${eventId}/submission-links`);
  return Array.isArray(payload?.links) ? payload.links : [];
}

export async function createSubmissionLink(eventId: number, payload: { label?: string; photographerName?: string; expiresInHours?: number; maxUses?: number }) {
  const result = await request<{ link: SubmissionLink }>(`/api/v1/admin/events/${eventId}/submission-links`, {
    method: 'POST',
    body: JSON.stringify(payload)
  });
  return result.link;
}

export async function revokeSubmissionLink(eventId: number, linkId: number) {
  return request<{ message: string }>(`/api/v1/admin/events/${eventId}/submission-links/${linkId}`, { method: 'DELETE' });
}

export async function deleteRevokedSubmissionLink(eventId: number, linkId: number) {
  return request<{ message: string }>(`/api/v1/admin/events/${eventId}/submission-links/${linkId}/record`, { method: 'DELETE' });
}

export async function approveSubmissions(submissionIds: number[], visibility?: Photo['visibility']) {
  const result = await request<{ processed: number; message: string }>('/api/v1/admin/submissions/batch-approve', {
    method: 'POST',
    body: JSON.stringify({ submissionIds, visibility: visibility || 'public' })
  });
  invalidateCacheTags(EVENTS_CACHE_TAG, ADMIN_DASHBOARD_CACHE_TAG);
  return result;
}

export async function deleteSubmissions(submissionIds: number[], headers?: Record<string, string>) {
  const result = await request<{ processed: number; message: string }>('/api/v1/admin/submissions/batch-delete', {
    method: 'POST',
    body: JSON.stringify({ submissionIds }),
    headers
  });
  invalidateCacheTags(ADMIN_DASHBOARD_CACHE_TAG);
  return result;
}

export async function getAdminSettings(): Promise<AdminSettingsResponse> {
  return request<AdminSettingsResponse>('/api/v1/admin/settings', undefined, {
    dedupe: true,
    tags: [ADMIN_SETTINGS_CACHE_TAG],
    ttlMs: 3_000
  });
}

export async function updateSiteSettings(site: SiteSettings) {
  const result = await request<{ site: SiteSettings; message: string }>('/api/v1/admin/settings/site', {
    method: 'PUT',
    body: JSON.stringify(site)
  });
  invalidateCacheTags(SITE_CACHE_TAG, ADMIN_SETTINGS_CACHE_TAG);
  return result;
}

export async function updateUploadSettings(upload: UploadSettings) {
  const result = await request<{ upload: UploadSettings; message: string }>('/api/v1/admin/settings/upload', {
    method: 'PUT',
    body: JSON.stringify(upload)
  });
  invalidateCacheTags(ADMIN_SETTINGS_CACHE_TAG);
  return result;
}

export async function getOIDCStatus() {
  return request<OIDCStatus>('/api/v1/admin/oidc/status');
}

export async function getOIDCBindURL() {
  return request<{ url: string }>('/api/v1/admin/oidc/bind', { method: 'POST' });
}

export async function unbindOIDC() {
  return request<{ message: string }>('/api/v1/admin/oidc/bind', { method: 'DELETE' });
}

export async function uploadSiteLogo(file: File) {
  const form = new FormData();
  form.append('file', file);
  const result = await request<{ site: SiteSettings; url: string; message: string }>('/api/v1/admin/settings/site/logo', {
    method: 'POST',
    body: form
  });
  invalidateCacheTags(SITE_CACHE_TAG, ADMIN_SETTINGS_CACHE_TAG);
  return result;
}

export async function clearSiteLogo() {
  const result = await request<{ site: SiteSettings; message: string }>('/api/v1/admin/settings/site/logo', {
    method: 'DELETE'
  });
  invalidateCacheTags(SITE_CACHE_TAG, ADMIN_SETTINGS_CACHE_TAG);
  return result;
}

export async function uploadSiteBackground(variant: 'desktop' | 'mobile', file: File) {
  const form = new FormData();
  form.append('file', file);
  const result = await request<{ site: SiteSettings; url: string; message: string; width: number; height: number }>(`/api/v1/admin/settings/site/background/${variant}`, {
    method: 'POST',
    body: form
  });
  invalidateCacheTags(SITE_CACHE_TAG, ADMIN_SETTINGS_CACHE_TAG);
  return result;
}

export async function clearSiteBackground(variant: 'desktop' | 'mobile') {
  const result = await request<{ site: SiteSettings; message: string }>(`/api/v1/admin/settings/site/background/${variant}`, {
    method: 'DELETE'
  });
  invalidateCacheTags(SITE_CACHE_TAG, ADMIN_SETTINGS_CACHE_TAG);
  return result;
}

export async function getAdminDashboard() {
  return request<{ stats: Record<string, number> }>('/api/v1/admin/dashboard', undefined, {
    dedupe: true,
    tags: [ADMIN_DASHBOARD_CACHE_TAG],
    ttlMs: 3_000
  });
}

export async function updateStoragePolicies(payload: { activePolicyId: string; policies: StoragePolicy[] }) {
  const result = await request<{ storagePolicies: { activePolicyId: string; policies: StoragePolicy[] }; usage: Record<string, StoragePolicyUsage>; message: string }>('/api/v1/admin/settings/storage', {
    method: 'PUT',
    body: JSON.stringify(payload)
  });
  invalidateCacheTags(ADMIN_SETTINGS_CACHE_TAG);
  return result;
}

export async function testStorageConnection(policy: StoragePolicy) {
  return request<{ success: boolean; error?: string }>('/api/v1/admin/settings/storage/test', {
    method: 'POST',
    body: JSON.stringify(policy)
  });
}

export async function scanOrphanStorageObjects() {
  return request<{ items: { policyId: string; key: string; size?: number }[]; total: number; totalSizeBytes: number; scannedPolicies: number; skippedPolicies: number; truncated: boolean }>('/api/v1/admin/maintenance/storage/orphans');
}

export async function scanMissingThumbnails() {
  return request<{ items: { id: number; eventId: number; kind: string; storagePolicyId: string; objectKey: string; contentType: string }[]; total: number; truncated: boolean }>('/api/v1/admin/maintenance/storage/missing-thumbnails');
}

export async function changePassword(currentPassword: string, newPassword: string) {
  return request<{ message: string }>('/api/v1/admin/change-password', {
    method: 'POST',
    body: JSON.stringify({ currentPassword, newPassword })
  });
}
