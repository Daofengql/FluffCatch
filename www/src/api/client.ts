export type EventCard = {
  id: number;
  slug: string;
  title: string;
  description: string;
  location: string;
  startTime?: string;
  endTime?: string;
  coverPolicyId?: string;
  coverObjectKey?: string;
  coverUrl?: string;
  isPublic: boolean;
  submissionEnabled: boolean;
};

export type Photo = {
  id: number;
  eventId: number;
  storagePolicyId: string;
  objectKey: string;
  url: string;
  thumbnailUrl?: string;
  originalFilename: string;
  photographerName?: string;
  visibility: 'public' | 'protected' | 'private';
  tags: { id: number; name: string }[];
};

export type Submission = {
  id: number;
  eventId: number;
  storagePolicyId: string;
  objectKey: string;
  originalFilename: string;
  photographerName?: string;
  tags: string[];
  status: string;
  createdAt: string;
};

export type StoragePolicyUsage = {
  policyId: string;
  objectCount: number;
  sizeBytes: number;
};

export type StoragePolicy = {
  id: string;
  name: string;
  driver: string;
  localPath?: string;
  publicPrefix: string;
  publicBaseUrl?: string;
};

export type SiteSettings = {
  name: string;
  subtitle: string;
  logoUrl: string;
};

export type MeResponse = {
  authenticated: boolean;
  username?: string;
};

export type AdminSettingsResponse = {
  settings: {
    site: SiteSettings;
    storagePolicies: {
      activePolicyId: string;
      policies: StoragePolicy[];
    };
  };
  usage: Record<string, StoragePolicyUsage>;
};

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    credentials: 'include',
    ...options,
    headers: {
      ...(options?.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
      ...options?.headers
    }
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error || response.statusText);
  }
  return (await response.json()) as T;
}

export type CaptchaChallenge = {
  id: string;
  imageSvg: string;
  expiresAt: string;
};

export async function getCaptcha() {
  return request<CaptchaChallenge>('/api/v1/auth/captcha');
}

export async function getSiteSettings(): Promise<SiteSettings> {
  return request<SiteSettings>('/api/v1/site');
}

export async function login(username: string, password: string, captchaId: string, captchaAnswer: string) {
  return request<{ authenticated: boolean; username?: string; message: string }>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password, captchaId, captchaAnswer })
  });
}

export async function logout() {
  return request<{ message: string }>('/api/v1/auth/logout', { method: 'POST' });
}

export async function getMe() {
  return request<MeResponse>('/api/v1/auth/me');
}

export async function getEvents(admin = false): Promise<EventCard[]> {
  const payload = await request<{ events: EventCard[] }>(admin ? '/api/v1/admin/events' : '/api/v1/events');
  return payload.events ?? [];
}

export async function getEvent(id: number): Promise<EventCard> {
  const payload = await request<{ event: EventCard }>(`/api/v1/events/${id}`);
  return payload.event;
}

export async function getPhotos(eventId: number, admin = false): Promise<Photo[]> {
  const payload = await request<{ photos: Photo[] }>(admin ? `/api/v1/admin/events/${eventId}/photos` : `/api/v1/events/${eventId}/photos`);
  return payload.photos ?? [];
}

export async function saveEvent(event: Partial<EventCard> & { submissionPassword?: string }) {
  const body = JSON.stringify({
    slug: event.slug,
    title: event.title,
    description: event.description,
    location: event.location,
    startTime: event.startTime || '',
    endTime: event.endTime || '',
    coverPolicyId: event.coverPolicyId || '',
    coverObjectKey: event.coverObjectKey || '',
    isPublic: Boolean(event.isPublic),
    submissionEnabled: event.submissionEnabled ?? true,
    submissionPassword: event.submissionPassword || ''
  });
  if (event.id) {
    return request<{ event: EventCard }>(`/api/v1/admin/events/${event.id}`, { method: 'PUT', body });
  }
  return request<{ event: EventCard }>('/api/v1/admin/events', { method: 'POST', body });
}

export async function uploadEventCover(eventId: number, file: File) {
  const form = new FormData();
  form.append('file', file);
  return request<{ policyId: string; objectKey: string; url: string }>(`/api/v1/admin/events/${eventId}/cover`, {
    method: 'POST',
    body: form
  });
}

export async function deleteEvent(eventId: number) {
  return request<{ message: string; deletedObjects: number }>(`/api/v1/admin/events/${eventId}`, {
    method: 'DELETE'
  });
}

export async function submitPhotos(eventId: number, form: FormData) {
  return request<{ submissions: Submission[] }>(`/api/v1/events/${eventId}/submissions`, {
    method: 'POST',
    body: form
  });
}

export async function getPendingSubmissions(): Promise<Submission[]> {
  const payload = await request<{ submissions: Submission[] }>('/api/v1/admin/submissions');
  return payload.submissions ?? [];
}

export async function approveSubmissions(submissionIds: number[]) {
  return request<{ processed: number; message: string }>('/api/v1/admin/submissions/batch-approve', {
    method: 'POST',
    body: JSON.stringify({ submissionIds })
  });
}

export async function deleteSubmissions(submissionIds: number[]) {
  return request<{ processed: number; message: string }>('/api/v1/admin/submissions/batch-delete', {
    method: 'POST',
    body: JSON.stringify({ submissionIds })
  });
}

export async function getAdminSettings(): Promise<AdminSettingsResponse> {
  return request<AdminSettingsResponse>('/api/v1/admin/settings');
}

export async function updateSiteSettings(site: SiteSettings) {
  return request<{ site: SiteSettings; message: string }>('/api/v1/admin/settings/site', {
    method: 'PUT',
    body: JSON.stringify(site)
  });
}

export async function getAdminDashboard() {
  return request<{ stats: Record<string, number> }>('/api/v1/admin/dashboard');
}
