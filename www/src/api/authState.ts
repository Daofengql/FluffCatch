import { getMe, login, logout, type MeResponse } from './client';

const AUTH_STORAGE_KEY = 'fluffcatch.authenticated';
const AUTH_EVENT = 'fluffcatch-auth-change';

let cachedMe: MeResponse | null = readCachedMe();
let pendingMe: Promise<MeResponse> | null = null;

function readCachedMe(): MeResponse {
  return {
    authenticated: window.localStorage.getItem(AUTH_STORAGE_KEY) === 'true'
  };
}

function writeCachedMe(next: MeResponse) {
  cachedMe = next;
  if (next.authenticated) {
    window.localStorage.setItem(AUTH_STORAGE_KEY, 'true');
  } else {
    window.localStorage.removeItem(AUTH_STORAGE_KEY);
  }
  window.dispatchEvent(new CustomEvent(AUTH_EVENT, { detail: next }));
}

export function getCachedMe() {
  return cachedMe ?? readCachedMe();
}

export function subscribeAuthState(listener: (next: MeResponse) => void) {
  const handler = (event: Event) => {
    listener((event as CustomEvent<MeResponse>).detail);
  };
  window.addEventListener(AUTH_EVENT, handler);
  return () => window.removeEventListener(AUTH_EVENT, handler);
}

export async function refreshMe(force = false) {
  if (!force && pendingMe) {
    return pendingMe;
  }

  pendingMe = getMe()
    .then((next) => {
      writeCachedMe(next);
      return next;
    })
    .catch((error) => {
      writeCachedMe({ authenticated: false });
      throw error;
    })
    .finally(() => {
      pendingMe = null;
    });

  return pendingMe;
}

export async function loginAdmin(username: string, password: string, captchaId: string, captchaAnswer: string) {
  const result = await login(username, password, captchaId, captchaAnswer);
  writeCachedMe({ authenticated: result.authenticated, username: result.username || username });
  return result;
}

export async function logoutAdmin() {
  try {
    return await logout();
  } finally {
    writeCachedMe({ authenticated: false });
  }
}
