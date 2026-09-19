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
  /** 当前用户是否已绑定企业微信账号 */
  wecomBound?: boolean;
};

type LoginResponse = {
  token: string;
  expires_at: string;
  wecom_bound?: boolean;
  user: AuthUser;
};

type ApiErrorResponse = {
  error?: string;
  message?: string;
};

/** 绑定弹窗向主窗口通知结果的 postMessage 类型标识 */
export const WECOM_BIND_MESSAGE = 'kvm:wecom-bind';

/** 企业微信认证配置保存后派发的变更事件，布局据此刷新绑定入口显隐 */
export const WECOM_PROVIDERS_CHANGED_EVENT = 'kvm:wecom-providers-changed';

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
  const user = {
    ...session.user,
    wecomBound: session.wecom_bound ?? session.user.wecomBound ?? false,
  };
  window.localStorage.setItem(USER_KEY, JSON.stringify(user));
  window.localStorage.setItem(EXPIRES_AT_KEY, session.expires_at);
  markSessionActivity();
}

/** 就地更新本地会话中的用户信息（如绑定状态变化），返回更新后的用户 */
export function updateStoredUser(patch: Partial<AuthUser>): AuthUser | null {
  const user = getStoredUser();
  if (!user) return null;
  const next = { ...user, ...patch };
  window.localStorage.setItem(USER_KEY, JSON.stringify(next));
  return next;
}

/** 获取企业微信扫码登录地址（公开接口：直连返回企微授权页，统一认证中心返回认证中心地址） */
export async function fetchWecomLoginUrl(redirect = '/') {
  const response = await fetch(
    `/api/auth/wecom/authorize?redirect=${encodeURIComponent(redirect)}`
  );
  if (!response.ok) {
    throw new Error(await readApiError(response));
  }
  const data = (await response.json()) as { url: string };
  return data.url;
}

/** 获取当前用户的企业微信绑定跳转地址（直连或统一认证中心按启用情况自动分派） */
export async function fetchWecomBindUrl() {
  const response = await fetch('/api/auth/wecom/bind-url', {
    headers: { Authorization: `Bearer ${getAuthToken()}` },
  });
  if (!response.ok) {
    throw new Error(await readApiError(response));
  }
  const data = (await response.json()) as { url: string };
  return data.url;
}

/** 解除当前用户的企业微信绑定，返回被解绑的企微账号（未绑定为 -） */
export async function unbindWecom() {
  const response = await fetch('/api/auth/wecom/bind', {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${getAuthToken()}` },
  });
  if (!response.ok) {
    throw new Error(await readApiError(response));
  }
  const data = (await response.json()) as { status: string; userid: string };
  return data.userid;
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
