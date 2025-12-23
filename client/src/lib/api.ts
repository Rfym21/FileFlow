const API_BASE = "/api";

/**
 * 获取存储的 JWT Token
 */
function getToken(): string | null {
  return localStorage.getItem("token");
}

/**
 * 设置 JWT Token
 */
export function setToken(token: string): void {
  localStorage.setItem("token", token);
}

/**
 * 清除 JWT Token
 */
export function clearToken(): void {
  localStorage.removeItem("token");
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
    throw new Error(error.error || "请求失败");
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
}

export async function login(data: LoginRequest): Promise<LoginResponse> {
  const result = await request<LoginResponse>("/auth/login", {
    method: "POST",
    body: JSON.stringify(data),
  });
  setToken(result.token);
  return result;
}

export async function checkAuth(): Promise<{ valid: boolean; user?: string }> {
  return request("/check");
}

// ==================== 账户 API ====================

export interface Quota {
  maxSizeBytes: number;
  maxClassAOps: number;
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
  s3: boolean;
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
export async function getAccounts(): Promise<AccountFull[]> {
  return request("/accounts");
}

/**
 * 分页获取账户列表
 */
export async function getAccountsPaged(
  page: number,
  pageSize: number
): Promise<PagedResponse<AccountFull>> {
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
}

export interface TokenRequest {
  name: string;
  permissions: string[];
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
  idGroup?: string[]
): Promise<AccountFiles[]> {
  const params = new URLSearchParams();
  if (prefix) params.set("prefix", prefix);
  if (cursor) params.set("cursor", cursor);
  if (limit) params.set("limit", limit.toString());
  if (idGroup && idGroup.length > 0) {
    params.set("idGroup", idGroup.join(","));
  }
  const query = params.toString() ? `?${params.toString()}` : "";
  return request(`/files${query}`);
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
  expirationDays?: number
): Promise<UploadResult> {
  const formData = new FormData();
  formData.append("file", file);
  if (path) formData.append("path", path);
  if (idGroup) formData.append("idGroup", idGroup);
  if (expirationDays !== undefined) {
    formData.append("expirationDays", expirationDays.toString());
  }

  return request("/upload", {
    method: "POST",
    body: formData,
  });
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
  s3VirtualHostedStyle: boolean;
  s3BaseDomain: string;
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

// ==================== S3 凭证 API ====================

export interface S3Credential {
  id: string;
  accessKeyId: string;
  secretAccessKey: string;
  accountId: string;
  description: string;
  permissions: string[];
  isActive: boolean;
  createdAt: string;
  lastUsedAt: string;
}

export interface S3CredentialRequest {
  accountId: string;
  description: string;
  permissions: string[];
}

export interface S3CredentialUpdateRequest {
  description: string;
  permissions: string[];
  isActive: boolean;
}

export async function getS3Credentials(): Promise<S3Credential[]> {
  const result = await request<{ credentials: S3Credential[] }>("/s3-credentials");
  return result.credentials;
}

export async function createS3Credential(data: S3CredentialRequest): Promise<{ credential: S3Credential }> {
  return request("/s3-credentials", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updateS3Credential(id: string, data: S3CredentialUpdateRequest): Promise<void> {
  return request(`/s3-credentials/${id}`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function deleteS3Credential(id: string): Promise<void> {
  return request(`/s3-credentials/${id}`, { method: "DELETE" });
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
}

export interface WebDAVCredentialRequest {
  accountId: string;
  description: string;
  permissions: string[];
  username?: string;
  password?: string;
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
