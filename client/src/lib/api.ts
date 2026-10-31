const API_BASE = "/api";

export const AUTH_CHANGED_EVENT = "fileflow-auth-changed";
export const PERMISSIONS_CHANGED_EVENT = "fileflow-permissions-changed";

export class APIError extends Error {
  status: number;
  code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
  }
}

/**
 * 获取存储的 JWT Token
 */
export function getToken(): string | null {
  return localStorage.getItem("token");
}

/**
 * 设置 JWT Token
 */
export function setToken(token: string): void {
  localStorage.setItem("token", token);
  window.dispatchEvent(new Event(AUTH_CHANGED_EVENT));
}

/**
 * 清除 JWT Token
 */
export function clearToken(): void {
  localStorage.removeItem("token");
  window.dispatchEvent(new Event(AUTH_CHANGED_EVENT));
}

/**
 * 检查是否已登录
 */
export function isLoggedIn(): boolean {
  return !!getToken();
}

/**
 * API 请求封装
 */
async function request<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();
  const headers: HeadersInit = {
    ...options.headers,
  };

  if (token) {
    (headers as Record<string, string>)["Authorization"] = `Bearer ${token}`;
  }

  if (!(options.body instanceof FormData)) {
    (headers as Record<string, string>)["Content-Type"] = "application/json";
  }

  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "请求失败" }));
    if (token && getToken() === token) {
      if (response.status === 401) clearToken();
      if (response.status === 403) {
        window.dispatchEvent(new Event(PERMISSIONS_CHANGED_EVENT));
      }
    }
    throw new APIError(error.error || "请求失败", response.status, error.code);
  }

  return response.json();
}

// ==================== 认证 API ====================

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  profile: UserProfile;
  permissions: Permission[];
}

export type TeamRole = "owner" | "admin" | "member" | "viewer";
export type Permission = "read" | "write" | "delete" | "manage";

export interface UserProfile {
  id: string;
  username: string;
  displayName: string;
  role: TeamRole;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

export const TEAM_ROLES: Record<TeamRole, { label: string; description: string }> = {
  owner: { label: "所有者", description: "管理团队、管理员和全部文件" },
  admin: { label: "管理员", description: "管理成员、存储配置和全部文件" },
  member: { label: "可读写成员", description: "浏览、上传、删除文件和修改到期时间" },
  viewer: { label: "只读成员", description: "浏览、预览和下载文件" },
};

export function rolePermissions(role: TeamRole): Permission[] {
  if (role === "owner" || role === "admin") return ["read", "write", "delete", "manage"];
  if (role === "member") return ["read", "write", "delete"];
  return ["read"];
}

export interface AuthResponse {
  valid: boolean;
  authType?: string;
  user?: string;
  profile?: UserProfile;
  permissions?: Permission[];
}

export async function login(data: LoginRequest): Promise<LoginResponse> {
  const result = await request<LoginResponse>("/auth/login", {
    method: "POST",
    body: JSON.stringify(data),
  });
  setToken(result.token);
  return result;
}

export async function checkAuth(): Promise<AuthResponse> {
  return request("/check");
}

export async function logout(): Promise<void> {
  await request("/auth/logout", { method: "POST" });
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  await request("/auth/password", {
    method: "POST",
    body: JSON.stringify({ currentPassword, newPassword }),
  });
}

export interface UserRequest {
  username: string;
  displayName: string;
  password: string;
  role: Exclude<TeamRole, "owner">;
}

export interface UserUpdateRequest {
  displayName: string;
  role: TeamRole;
  isActive: boolean;
  password?: string;
}

export async function getUsers(): Promise<UserProfile[]> {
  const result = await request<{ users: UserProfile[] }>("/users");
  return result.users || [];
}

export async function createUser(data: UserRequest): Promise<UserProfile> {
  return request("/users", { method: "POST", body: JSON.stringify(data) });
}

export async function updateUser(id: string, data: UserUpdateRequest): Promise<UserProfile> {
  return request(`/users/${encodeURIComponent(id)}`, {
    method: "PUT", body: JSON.stringify(data),
  });
}

export interface AuditEntry {
  id: string;
  userId: string;
  username: string;
  action: string;
  target: string;
  status: number;
  createdAt: string;
}

export async function getAuditLogs(): Promise<AuditEntry[]> {
  const result = await request<{ entries: AuditEntry[] }>("/audit-logs?limit=100");
  return result.entries || [];
}

// ==================== 账户 API ====================

export interface Quota {
  maxSizeBytes: number;
  maxClassAOps: number;
  maxClassBOps?: number;
}

export interface Usage {
  sizeBytes: number;
  classAOps: number;
  classBOps: number;
  lastSyncAt: string;
}

/**
 * 账户权限配置
 */
export interface AccountPermissions {
  webdav: boolean;
  autoUpload: boolean;
  apiUpload: boolean;
  clientUpload: boolean;
}

export interface Account {
  id: string;
  name: string;
  isActive: boolean;
  description: string;
  accountId: string;
  bucketName: string;
  endpoint: string;
  publicDomain: string;
  hasApiToken: boolean;
  quota: Quota;
  usage: Usage;
  permissions: AccountPermissions;
  usagePercent: number;
  isOverQuota: boolean;
  isOverOps: boolean;
  isAvailable: boolean;
  createdAt: string;
  updatedAt: string;
}

/**
 * 账户完整信息（包含敏感字段，用于创建/编辑响应）
 */
export interface AccountFull {
  id: string;
  name: string;
  isActive: boolean;
  description: string;
  accountId: string;
  accessKeyId: string;
  secretAccessKey: string;
  bucketName: string;
  endpoint: string;
  publicDomain: string;
  apiToken: string;
  hasApiToken?: boolean; // 兼容性字段，可选
  quota: Quota;
  usage: Usage;
  permissions: AccountPermissions;
  usagePercent: number;
  isOverQuota: boolean;
  isOverOps: boolean;
  isAvailable: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface AccountRequest {
  name: string;
  isActive: boolean;
  description: string;
  accountId: string;
  accessKeyId: string;
  secretAccessKey: string;
  bucketName: string;
  endpoint: string;
  publicDomain: string;
  apiToken: string;
  quota: Quota;
  permissions: AccountPermissions;
}

// ==================== 分页相关类型 ====================

export interface PagedResponse<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

export interface AccountsStats {
  totalAccounts: number;
  availableCount: number;
  totalSizeBytes: number;
  totalQuotaBytes: number;
  totalWriteOps: number;
  totalReadOps: number;
}

/**
 * 获取所有账户（不分页，兼容旧接口）
 */
export async function getAccounts<T = AccountFull>(): Promise<T[]> {
  return request("/accounts");
}

/**
 * 分页获取账户列表
 */
export async function getAccountsPaged<T = AccountFull>(
  page: number,
  pageSize: number
): Promise<PagedResponse<T>> {
  return request(`/accounts?page=${page}&pageSize=${pageSize}`);
}

/**
 * 获取账户统计信息
 */
export async function getAccountsStats(): Promise<AccountsStats> {
  return request("/accounts/stats");
}

export async function getAccount(id: string): Promise<AccountFull> {
  return request(`/accounts/${id}`);
}

export async function createAccount(data: AccountRequest): Promise<AccountFull> {
  return request("/accounts", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updateAccount(
  id: string,
  data: AccountRequest
): Promise<AccountFull> {
  return request(`/accounts/${id}`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function deleteAccount(id: string): Promise<void> {
  return request(`/accounts/${id}`, { method: "DELETE" });
}

/**
 * 同步账户使用量
 * @param accountId 可选，指定账户 ID 则同步单个账户，否则同步所有账户
 * @returns 同步所有账户时返回账户列表，同步单个账户时返回该账户信息
 */
export async function syncAccounts(accountId?: string): Promise<AccountFull[] | Account> {
  const query = accountId ? `?accountId=${accountId}` : "";
  return request(`/accounts/sync${query}`, { method: "POST" });
}

export async function clearBucket(accountId: string): Promise<void> {
  return request(`/accounts/${accountId}/clear`, { method: "POST" });
}

// ==================== 批量删除旧文件 API ====================

export interface DeleteOldFilesResult {
  accountId: string;
  accountName: string;
  deletedCount: number;
  error?: string;
}

export interface DeleteOldFilesResponse {
  results: DeleteOldFilesResult[];
}

/**
 * 批量删除指定账户中早于指定日期的文件
 * @param accountIds 账户 ID 列表
 * @param beforeDate 日期字符串，格式为 YYYY-MM-DD
 */
export async function deleteOldFiles(
  accountIds: string[],
  beforeDate: string
): Promise<DeleteOldFilesResponse> {
  return request("/accounts/delete-old-files", {
    method: "POST",
    body: JSON.stringify({ accountIds, beforeDate }),
  });
}

// ==================== Token API ====================

export interface Token {
  id: string;
  name: string;
  token?: string;
  permissions: string[];
  createdAt: string;
  userId: string;
}

export interface TokenRequest {
  name: string;
  permissions: string[];
  userId?: string;
}

export async function getTokens(): Promise<Token[]> {
  return request("/tokens");
}

export async function createToken(data: TokenRequest): Promise<Token> {
  return request("/tokens", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function deleteToken(id: string): Promise<void> {
  return request(`/tokens/${id}`, { method: "DELETE" });
}

// ==================== 文件 API ====================

export interface FileNode {
  key: string;
  name: string;
  size?: number;
  lastModified?: string;
  isDir: boolean;
  children?: FileNode[];
}

export interface AccountFiles {
  id: string;
  accountName: string;
  files: FileNode[];
  sizeBytes: number;
  maxSize: number;
  nextCursor?: string;
}

export interface UploadResult {
  id: string;
  accountName: string;
  key: string;
  size: number;
  url: string;
}

export async function getFiles(
  prefix?: string,
  cursor?: string,
  limit?: number,
  idGroup?: string[],
  options?: { flat?: boolean; query?: string; signal?: AbortSignal }
): Promise<AccountFiles[]> {
  const params = new URLSearchParams();
  if (prefix) params.set("prefix", prefix);
  if (cursor) params.set("cursor", cursor);
  if (limit) params.set("limit", limit.toString());
  if (options?.flat) params.set("flat", "true");
  if (options?.query) params.set("q", options.query);
  if (idGroup && idGroup.length > 0) {
    params.set("idGroup", idGroup.join(","));
  }
  const query = params.toString() ? `?${params.toString()}` : "";
  return request(`/files${query}`, { signal: options?.signal });
}

export interface UploadInitRequest {
  filename: string;
  size: number;
  contentType?: string;
  path?: string;
  idGroup?: string;
  expirationDays?: number;
  randomFilename?: boolean;
  requestId?: string;
}

export interface UploadInitResponse {
  uploadId: string;
  url: string;
  method: "PUT";
  headers: Record<string, string>;
  expiresAt: string;
  accountId: string;
  key: string;
  size: number;
}

export async function initDirectUpload(data: UploadInitRequest, signal?: AbortSignal): Promise<UploadInitResponse> {
  return request("/upload/init", { method: "POST", body: JSON.stringify(data), signal });
}

export async function completeDirectUpload(uploadId: string, signal?: AbortSignal): Promise<UploadResult> {
  return request("/upload/complete", { method: "POST", body: JSON.stringify({ uploadId }), signal });
}

type UploadTransferOptions = { signal?: AbortSignal; onProgress?: (progress: number) => void };

function sendUpload<T>(url: string, method: string, body: XMLHttpRequestBodyInit, headers: Record<string, string>, options: UploadTransferOptions, localAPI: boolean): Promise<T> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const token = localAPI ? getToken() : null;
    const cleanup = () => options.signal?.removeEventListener("abort", abort);
    const abort = () => xhr.abort();
    if (options.signal?.aborted) { reject(new DOMException("上传已取消", "AbortError")); return; }
    xhr.open(method, url);
    xhr.timeout = 60 * 60 * 1000;
    if (token) xhr.setRequestHeader("Authorization", `Bearer ${token}`);
    for (const [name, value] of Object.entries(headers)) {
      // The browser supplies these forbidden headers from the actual File.
      if (!["host", "content-length", "authorization"].includes(name.toLowerCase())) xhr.setRequestHeader(name, value);
    }
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable && event.total > 0) options.onProgress?.(Math.min(99, Math.round(event.loaded * 100 / event.total)));
    };
    xhr.onload = () => {
      cleanup();
      if (xhr.status >= 200 && xhr.status < 300) {
        try { resolve(localAPI ? JSON.parse(xhr.responseText) as T : undefined as T); }
        catch { reject(new Error("上传响应格式错误，请刷新文件列表确认结果")); }
        return;
      }
      let message = localAPI ? "服务器上传失败" : `R2 上传失败（${xhr.status}），请检查上传链接或存储桶设置`;
      if (localAPI) {
        try { message = JSON.parse(xhr.responseText).error || message; } catch { /* keep a concise HTTP error */ }
        if (token && getToken() === token) {
          if (xhr.status === 401) clearToken();
          if (xhr.status === 403) window.dispatchEvent(new Event(PERMISSIONS_CHANGED_EVENT));
        }
      }
      reject(new APIError(message, xhr.status));
    };
    xhr.onerror = () => { cleanup(); reject(new Error(localAPI ? "网络连接失败，请检查服务器连接" : "直传连接失败，请检查网络和 R2 存储桶的 CORS 配置，或选择服务器中转上传")); };
    xhr.ontimeout = () => { cleanup(); reject(new Error("上传超时，请检查连接后重试")); };
    xhr.onabort = () => { cleanup(); reject(new DOMException("上传已取消", "AbortError")); };
    options.signal?.addEventListener("abort", abort, { once: true });
    xhr.send(body);
  });
}

export async function putDirectUpload(session: UploadInitResponse, file: File, options: UploadTransferOptions = {}): Promise<void> {
  if (session.size !== file.size) throw new Error("上传任务与文件大小不一致");
  await sendUpload<void>(session.url, session.method, file, session.headers, options, false);
}

/**
 * 上传文件
 * @param file 要上传的文件
 * @param path 自定义路径（可选）
 * @param idGroup 目标账户 ID（可选，不指定则智能选择）
 * @param expirationDays 到期天数（可选，-1=使用默认设置，0=永久，>0=指定天数）
 */
export async function uploadFile(
  file: File,
  path?: string,
  idGroup?: string,
  expirationDays?: number,
  randomFilename: boolean = true,
  options: UploadTransferOptions = {}
): Promise<UploadResult> {
  const formData = new FormData();
  formData.append("file", file);
  formData.append("randomFilename", String(randomFilename));
  if (path) formData.append("path", path);
  if (idGroup) formData.append("idGroup", idGroup);
  if (expirationDays !== undefined) {
    formData.append("expirationDays", expirationDays.toString());
  }

  return sendUpload<UploadResult>(`${API_BASE}/upload`, "POST", formData, {}, options, true);
}

export interface BatchDeleteFilesResponse {
  results: { key: string; success: boolean; error?: string }[];
  successCount: number;
  failCount: number;
}

export async function batchDeleteFiles(accountId: string, keys: string[]): Promise<BatchDeleteFilesResponse> {
  return request("/files/batch-delete", { method: "POST", body: JSON.stringify({ accountId, keys }) });
}

export async function setFilesExpiration(accountId: string, keys: string[], expiresAt: string | null): Promise<{ updatedCount: number }> {
  return request("/files/expirations", { method: "PATCH", body: JSON.stringify({ accountId, keys, expiresAt }) });
}

export async function deleteFile(
  idGroup: string,
  key: string
): Promise<void> {
  return request(`/file?idGroup=${idGroup}&key=${encodeURIComponent(key)}`, {
    method: "DELETE",
  });
}

export async function getFileLink(
  idGroup: string,
  key: string
): Promise<{ url: string }> {
  return request(
    `/link?idGroup=${idGroup}&key=${encodeURIComponent(key)}`
  );
}

// ==================== 系统设置 API ====================

export interface Settings {
  syncInterval: number;
  endpointProxy: boolean;
  endpointProxyUrl: string;
  defaultExpirationDays: number;
  expirationCheckMinutes: number;
}

export async function getSettings(): Promise<Settings> {
  return request("/settings");
}

export async function updateSettings(data: Settings): Promise<Settings> {
  return request("/settings", {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

// ==================== WebDAV 凭证 API ====================

export interface WebDAVCredential {
  id: string;
  username: string;
  password: string;
  accountId: string;
  description: string;
  permissions: string[];
  isActive: boolean;
  createdAt: string;
  lastUsedAt: string;
  userId: string;
}

export interface WebDAVCredentialRequest {
  accountId: string;
  description: string;
  permissions: string[];
  username?: string;
  password?: string;
  userId?: string;
}

export interface WebDAVCredentialUpdateRequest {
  description: string;
  permissions: string[];
  isActive: boolean;
}

export async function getWebDAVCredentials(): Promise<WebDAVCredential[]> {
  const result = await request<{ credentials: WebDAVCredential[] }>("/webdav-credentials");
  return result.credentials;
}

export async function createWebDAVCredential(data: WebDAVCredentialRequest): Promise<{ credential: WebDAVCredential }> {
  return request("/webdav-credentials", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updateWebDAVCredential(id: string, data: WebDAVCredentialUpdateRequest): Promise<void> {
  return request(`/webdav-credentials/${id}`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function deleteWebDAVCredential(id: string): Promise<void> {
  return request(`/webdav-credentials/${id}`, { method: "DELETE" });
}

// ==================== 文件到期管理 API ====================

export interface FileExpiration {
  id: string;
  accountId: string;
  accountName: string;
  fileKey: string;
  expiresAt: string;
  createdAt: string;
}

export interface FileExpirationsResponse {
  expirations: FileExpiration[];
  total: number;
}

export async function getFileExpirations(): Promise<FileExpirationsResponse> {
  return request("/file-expirations");
}

export async function deleteFileExpiration(id: string): Promise<void> {
  return request(`/file-expirations/${id}`, { method: "DELETE" });
}

export interface UpdateFileExpirationsResponse {
  updatedCount: number;
}

export interface DeleteFileExpirationsResponse {
  results: { id: string; success: boolean; error?: string }[];
  successCount: number;
  failCount: number;
}

export async function updateFileExpiration(id: string, expiresAt: string): Promise<UpdateFileExpirationsResponse> {
  return request(`/file-expirations/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify({ expiresAt }),
  });
}

export async function updateFileExpirations(ids: string[], expiresAt: string): Promise<UpdateFileExpirationsResponse> {
  return request("/file-expirations", {
    method: "PATCH",
    body: JSON.stringify({ ids, expiresAt }),
  });
}

export async function deleteFileExpirations(ids: string[]): Promise<DeleteFileExpirationsResponse> {
  return request("/file-expirations/batch-delete", {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
}

export interface ZipPreviewEntry {
  name: string;
  size: number;
  compressedSize: number;
  modifiedAt?: string;
  isDir: boolean;
}

export interface FilePreview {
  url: string;
  downloadUrl?: string;
  contentType: string;
  kind: "image" | "video" | "audio" | "pdf" | "text" | "zip" | "unsupported";
  text?: string;
  truncated?: boolean;
  entries?: ZipPreviewEntry[];
  entryCount?: number;
  archiveError?: string;
}

export async function getFilePreview(accountId: string, key: string, signal?: AbortSignal): Promise<FilePreview> {
  const query = new URLSearchParams({ idGroup: accountId, key });
  return request(`/file-preview?${query.toString()}`, { signal });
}
