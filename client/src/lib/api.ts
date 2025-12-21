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
}

export async function getAccounts(): Promise<Account[]> {
  return request("/accounts");
}

export async function getAccount(id: string): Promise<Account> {
  return request(`/accounts/${id}`);
}

export async function createAccount(data: AccountRequest): Promise<Account> {
  return request("/accounts", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updateAccount(
  id: string,
  data: AccountRequest
): Promise<Account> {
  return request(`/accounts/${id}`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function deleteAccount(id: string): Promise<void> {
  return request(`/accounts/${id}`, { method: "DELETE" });
}

export async function syncAccounts(accountId?: string): Promise<void> {
  const query = accountId ? `?accountId=${accountId}` : "";
  return request(`/accounts/sync${query}`, { method: "POST" });
}

export async function clearBucket(accountId: string): Promise<void> {
  return request(`/accounts/${accountId}/clear`, { method: "POST" });
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

export async function uploadFile(
  file: File,
  path?: string,
  idGroup?: string
): Promise<UploadResult> {
  const formData = new FormData();
  formData.append("file", file);
  if (path) formData.append("path", path);
  if (idGroup) formData.append("idGroup", idGroup);

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
