/** 登录态存取层：令牌与用户信息持久化于 localStorage，同一浏览器新建标签页与重启后保持登录，仅随服务端会话过期失效 */
const TOKEN_KEY = 'kvm.auth.token';
const USER_KEY = 'kvm.auth.user';
const EXPIRES_AT_KEY = 'kvm.auth.expires_at';

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

export function userHasPermission(user: AuthUser | null, permission: string) {
  if (!user) return false;
  if (user.role === 'admin') return true;
  return user.permissions?.includes(permission) ?? false;
}

export function userHasAnyPermission(user: AuthUser | null, permissions: string[]) {
  return permissions.some(permission => userHasPermission(user, permission));
}

export function persistSession(session: LoginResponse) {
  clearAuthExpiredFlag();
  window.localStorage.setItem(TOKEN_KEY, session.token);
  const user = {
    ...session.user,
    wecomBound: session.wecom_bound ?? session.user.wecomBound ?? false,
  };
  window.localStorage.setItem(USER_KEY, JSON.stringify(user));
  window.localStorage.setItem(EXPIRES_AT_KEY, session.expires_at);
}

/** 就地更新本地会话中的用户信息（如绑定状态变化），返回更新后的用户 */
export function updateStoredUser(patch: Partial<AuthUser>): AuthUser | null {
  const user = getStoredUser();
  if (!user) return null;
  const next = { ...user, ...patch };
  window.localStorage.setItem(USER_KEY, JSON.stringify(next));
  return next;
}

/** WecomAuthorizeEmbed 内嵌二维码登录参数：iframe 地址、回跳中转路由与直连模式签发的 state */
export type WecomAuthorizeEmbed = {
  auth_mode: 'direct' | 'sso';
  iframe_url: string;
  callback_path: string;
  state?: string;
};

type WecomAuthorizeResponse = {
  url: string;
  embed?: WecomAuthorizeEmbed;
};

/** 获取企业微信扫码登录跳转地址与内嵌二维码参数（公开接口：直连返回企微授权页，统一认证中心返回认证中心地址） */
export async function fetchWecomAuthorize(redirect = '/') {
  const response = await fetch(
    `/api/auth/wecom/authorize?redirect=${encodeURIComponent(redirect)}`
  );
  if (!response.ok) {
    throw new Error(await readApiError(response));
  }
  return (await response.json()) as WecomAuthorizeResponse;
}

/** 内嵌扫码直连回调：以授权码与 state 换取登录会话 */
export async function loginWithWecomCode(body: { code: string; state: string }) {
  const response = await fetch('/api/auth/wecom/callback', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    throw new Error(await readApiError(response));
  }
  return (await response.json()) as LoginResponse;
}

/** 内嵌扫码统一认证中心回调：以票据换取登录会话 */
export async function loginWithWecomCenterTicket(ticket: string) {
  const response = await fetch('/api/auth/wecom/sso/callback', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ticket }),
  });
  if (!response.ok) {
    throw new Error(await readApiError(response));
  }
  return (await response.json()) as LoginResponse;
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
}

const AUTH_EXPIRED_FLAG_KEY = 'kvm.auth.expired';
let authRedirectingToLogin = false;

// markAuthExpired 标记本次进入登录页由会话失效引起：仅携带令牌的认证请求收到 401 时调用，
// 登录页挂载时消费该标记并提示重新登录；主动登出、未登录访问与业务型 401 豁免不产生标记
export function markAuthExpired() {
  try {
    sessionStorage.setItem(AUTH_EXPIRED_FLAG_KEY, '1');
  } catch {
    return;
  }
}

// consumeAuthExpired 读取并清除会话失效标记，返回是否存在待提示的过期进入
export function consumeAuthExpired() {
  try {
    const expired = sessionStorage.getItem(AUTH_EXPIRED_FLAG_KEY) === '1';
    if (expired) sessionStorage.removeItem(AUTH_EXPIRED_FLAG_KEY);
    return expired;
  } catch {
    return false;
  }
}

// clearAuthExpiredFlag 清除尚未消费的会话失效标记：登录成功与主动登出后残留标记不再引发误提示
export function clearAuthExpiredFlag() {
  try {
    sessionStorage.removeItem(AUTH_EXPIRED_FLAG_KEY);
  } catch {
    return;
  }
}

// markAuthRedirecting 标记正以整页刷新跳转登录页，路由守卫据此跳过客户端跳转，
// 避免"先无动画进入登录页、再整页刷新带加载动画"的重复进入
export function markAuthRedirecting() {
  authRedirectingToLogin = true;
}

// isAuthRedirecting 返回当前文档是否已触发会话失效的整页跳转
export function isAuthRedirecting() {
  return authRedirectingToLogin;
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

// logout 先清理本地会话再通知后端：退出引发的组件重挂载若触发重新请求，令牌已不在本地，
// 不会携带"服务端已注销"的旧令牌而误报会话过期；注销请求失败静默忽略，不阻塞退出流程
export async function logout() {
  const token = getAuthToken();
  clearAuthExpiredFlag();
  clearSession();

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
  }
}
