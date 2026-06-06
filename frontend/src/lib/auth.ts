const TOKEN_KEY = 'kvm.auth.token';
const USER_KEY = 'kvm.auth.user';
const EXPIRES_AT_KEY = 'kvm.auth.expires_at';
const LAST_ACTIVITY_AT_KEY = 'kvm.auth.last_activity_at';
const SESSION_IDLE_TIMEOUT_MS = 12 * 60 * 60 * 1000;

export type AuthUser = {
  id?: string;
  username: string;
  displayName: string;
  role: string;
  permissions?: string[];
};

type LoginResponse = {
  token: string;
  expires_at: string;
  user: AuthUser;
};

type ApiErrorResponse = {
  error?: string;
  message?: string;
};

export function getAuthToken() {
  return window.localStorage.getItem(TOKEN_KEY);
}

export function getStoredUser(): AuthUser | null {
  const raw = window.localStorage.getItem(USER_KEY);
  if (!raw) return null;

  try {
    return JSON.parse(raw) as AuthUser;
  } catch {
    window.localStorage.removeItem(USER_KEY);
    return null;
  }
}

export function isAuthenticated() {
  return Boolean(getAuthToken());
}

export function getSessionExpiresAt() {
  return window.localStorage.getItem(EXPIRES_AT_KEY);
}

export function getLastActivityAt() {
  const raw = window.localStorage.getItem(LAST_ACTIVITY_AT_KEY);
  const value = raw ? Number(raw) : 0;
  return Number.isFinite(value) ? value : 0;
}

export function markSessionActivity(now = Date.now()) {
  if (!getAuthToken()) return;
  window.localStorage.setItem(LAST_ACTIVITY_AT_KEY, String(now));
}

export function isSessionIdleExpired(now = Date.now()) {
  const lastActivityAt = getLastActivityAt();
  return lastActivityAt <= 0 || now - lastActivityAt >= SESSION_IDLE_TIMEOUT_MS;
}

export function userHasPermission(user: AuthUser | null, permission: string) {
  if (!user) return false;
  if (user.role === 'admin') return true;
  return user.permissions?.includes(permission) ?? false;
}

export function userHasAnyPermission(user: AuthUser | null, permissions: string[]) {
  return permissions.some(permission => userHasPermission(user, permission));
}

export function persistSession(session: LoginResponse) {
  window.localStorage.setItem(TOKEN_KEY, session.token);
  window.localStorage.setItem(USER_KEY, JSON.stringify(session.user));
  window.localStorage.setItem(EXPIRES_AT_KEY, session.expires_at);
  markSessionActivity();
}

export function clearSession() {
  window.localStorage.removeItem(TOKEN_KEY);
  window.localStorage.removeItem(USER_KEY);
  window.localStorage.removeItem(EXPIRES_AT_KEY);
  window.localStorage.removeItem(LAST_ACTIVITY_AT_KEY);
}

async function readApiError(response: Response) {
  try {
    const body = (await response.json()) as ApiErrorResponse;
    return body.message || body.error || `请求失败：${response.status}`;
  } catch {
    return `请求失败：${response.status}`;
  }
}

export async function login(username: string, password: string, provider = 'local') {
  const response = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password, provider }),
  });

  if (!response.ok) {
    throw new Error(await readApiError(response));
  }

  const session = (await response.json()) as LoginResponse;
  persistSession(session);
  return session;
}

export async function logout() {
  const token = getAuthToken();

  try {
    if (token) {
      const response = await fetch('/api/auth/logout', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!response.ok && response.status !== 401) {
        console.warn('Logout request failed:', response.status);
      }
    }
  } catch (error) {
    console.warn('Logout request failed:', error);
  } finally {
    clearSession();
  }
}
