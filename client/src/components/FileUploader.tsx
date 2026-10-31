import { useState, useRef, useCallback, useEffect } from "react";
import { toast } from "sonner";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { uploadFile, getAccounts, type UploadResult, type Account } from "@/lib/api";
import { formatBytes } from "@/lib/utils";
import { X, Copy, Check, FileImage, FileVideo, File as FileIcon, CloudUpload, ExternalLink, FolderOpen, RotateCcw } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/hooks/useAuth";

type UploadOptions = { path: string; accountId: string; expirationDays: number; randomFilename: boolean };
interface UploadItem {
  id: string;
  createdAt: number;
  file: File;
  options: UploadOptions;
  status: "pending" | "uploading" | "success" | "error" | "cancelled";
  progress: number;
  result?: UploadResult;
  error?: string;
  retryExpired?: boolean;
}
interface FileUploaderProps {
  onUploadComplete?: () => void;
  defaultAccountId?: string;
  defaultPath?: string;
  defaultExpirationDays?: number;
}
function isImageFile(file: File | string): boolean { return (typeof file === "string" ? file : file.type).startsWith("image/"); }
function isVideoFile(file: File | string): boolean { return (typeof file === "string" ? file : file.type).startsWith("video/"); }
const EXPIRATION_OPTIONS = [{ value: -1, label: "使用默认设置" }, { value: 1, label: "1 天" }, { value: 7, label: "7 天" }, { value: 30, label: "30 天" }, { value: 90, label: "90 天" }, { value: 365, label: "1 年" }, { value: 0, label: "永久" }, { value: -2, label: "自定义" }];
const MAX_FILE_BYTES = 5 * 1024 * 1024 * 1024;
const MAX_CONCURRENT_UPLOADS = 3;
const MAX_RETRY_WINDOW_MS = 24 * 60 * 60 * 1000;
function uploadRequestId(): string {
  if (typeof crypto.randomUUID === "function") return crypto.randomUUID();
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 15) | 64;
  bytes[8] = (bytes[8] & 63) | 128;
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
  return [hex.slice(0, 8), hex.slice(8, 12), hex.slice(12, 16), hex.slice(16, 20), hex.slice(20)].join("-");
}

export default function FileUploader({ onUploadComplete, defaultAccountId, defaultPath, defaultExpirationDays = -1 }: FileUploaderProps) {
  const { can } = useAuth();
  const [isDragging, setIsDragging] = useState(false);
  const [uploads, setUploads] = useState<UploadItem[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selectedAccountId, setSelectedAccountId] = useState(defaultAccountId || "");
  const [customPath, setCustomPath] = useState(defaultPath || "");
  const [randomFilename, setRandomFilename] = useState(true);
  const [expirationDays, setExpirationDays] = useState(defaultExpirationDays);
  const [customExpirationDays, setCustomExpirationDays] = useState("30");
  const [isLoadingAccounts, setIsLoadingAccounts] = useState(true);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const dragCounter = useRef(0);
  const uploadsRef = useRef<UploadItem[]>([]);
  const controllers = useRef(new Map<string, AbortController>());
  const activeCount = useRef(0);
  const mounted = useRef(true);
  const canWrite = useRef(can("write"));
  const onCompleteRef = useRef(onUploadComplete);

  useEffect(() => {
    mounted.current = true;
    const active = controllers.current;
    return () => { mounted.current = false; active.forEach((controller) => controller.abort()); active.clear(); };
  }, []);
  useEffect(() => { canWrite.current = can("write"); if (!canWrite.current) controllers.current.forEach((controller) => controller.abort()); }, [can]);
  useEffect(() => { onCompleteRef.current = onUploadComplete; }, [onUploadComplete]);
  useEffect(() => {
    let active = true;
    void getAccounts<Account>().then((data) => { if (active) setAccounts(data || []); }).catch(() => { if (active) toast.error("加载存储账户失败，请刷新后重试"); }).finally(() => { if (active) setIsLoadingAccounts(false); });
    return () => { active = false; };
  }, []);
  useEffect(() => { if (defaultAccountId) setSelectedAccountId(defaultAccountId); }, [defaultAccountId]);
  useEffect(() => { setCustomPath(defaultPath || ""); }, [defaultPath]);

  const updateItems = (items: UploadItem[]) => { uploadsRef.current = items; if (mounted.current) setUploads(items); };
  const updateItem = (id: string, patch: Partial<UploadItem>) => updateItems(uploadsRef.current.map((item) => item.id === id ? { ...item, ...patch } : item));
  const success = (id: string, result: UploadResult) => { updateItem(id, { status: "success", progress: 100, result, error: undefined }); if (mounted.current) onCompleteRef.current?.(); };

  const runUpload = async (item: UploadItem) => {
    const controller = new AbortController();
    controllers.current.set(item.id, controller);
    const signal = controller.signal;
    try {
      const result = await uploadFile(item.file, item.options.path || undefined, item.options.accountId || undefined, item.options.expirationDays, item.options.randomFilename, { signal, requestId: item.id, onProgress: (progress) => updateItem(item.id, { progress }) });
      success(item.id, result);
    } catch (error) {
      const cancelled = error instanceof DOMException && error.name === "AbortError";
      updateItem(item.id, { status: cancelled ? "cancelled" : "error", error: cancelled ? "上传已取消" : error instanceof Error ? error.message : "上传失败，请重试" });
    } finally {
      controllers.current.delete(item.id);
      activeCount.current -= 1;
      pump();
    }
  };
  const pump = () => {
    if (!mounted.current || !canWrite.current) return;
    while (activeCount.current < MAX_CONCURRENT_UPLOADS) {
      const item = uploadsRef.current.find((entry) => entry.status === "pending");
      if (!item) break;
      activeCount.current += 1;
      updateItem(item.id, { status: "uploading", error: undefined });
      void runUpload(item);
    }
  };
  const addFilesToUpload = (files: File[]) => {
    if (!can("write")) return;
    const days = expirationDays === -2 ? Number(customExpirationDays) : expirationDays;
    if (!Number.isInteger(days) || days < -1 || days > 3650 || (expirationDays === -2 && days < 1)) { toast.error("自定义有效期需为 1 到 3650 天"); return; }
    const options: UploadOptions = { path: customPath.trim(), accountId: selectedAccountId, expirationDays: days, randomFilename };
    const createdAt = Date.now();
    const added = files.map((file): UploadItem => ({ id: uploadRequestId(), createdAt, file, options: { ...options }, status: file.size > MAX_FILE_BYTES ? "error" : "pending", progress: 0, ...(file.size > MAX_FILE_BYTES ? { error: "单文件最大 5 GiB" } : {}) }));
    updateItems([...uploadsRef.current, ...added]);
    pump();
  };
  const cancelUpload = (id: string) => {
    const controller = controllers.current.get(id);
    if (controller) controller.abort();
    else updateItem(id, { status: "cancelled", error: "上传已取消" });
  };
  const removeUpload = (id: string) => { controllers.current.get(id)?.abort(); updateItems(uploadsRef.current.filter((item) => item.id !== id)); };
  const retryUpload = (id: string) => {
    if (!can("write") || controllers.current.has(id)) return;
    const item = uploadsRef.current.find((entry) => entry.id === id);
    if (!item || item.file.size > MAX_FILE_BYTES) return;
    if (Date.now() - item.createdAt >= MAX_RETRY_WINDOW_MS) {
      const error = "重试窗口已过，请先核对文件列表，再重新选择文件上传";
      updateItem(id, { status: "error", error, retryExpired: true });
      toast.error(error);
      return;
    }
    updateItem(id, { status: "pending", progress: 0, error: undefined });
    pump();
  };
  const handleDragEnter = useCallback((event: React.DragEvent) => { event.preventDefault(); event.stopPropagation(); dragCounter.current += 1; if (event.dataTransfer.items.length) setIsDragging(true); }, []);
  const handleDragLeave = useCallback((event: React.DragEvent) => { event.preventDefault(); event.stopPropagation(); dragCounter.current -= 1; if (dragCounter.current === 0) setIsDragging(false); }, []);
  const handleDragOver = useCallback((event: React.DragEvent) => { event.preventDefault(); event.stopPropagation(); }, []);
  const handleDrop = (event: React.DragEvent) => { event.preventDefault(); event.stopPropagation(); setIsDragging(false); dragCounter.current = 0; addFilesToUpload(Array.from(event.dataTransfer.files)); };
  const handleFileSelect = (event: React.ChangeEvent<HTMLInputElement>) => { addFilesToUpload(Array.from(event.target.files || [])); event.target.value = ""; };
  const availableAccounts = accounts.filter((account) => account.isAvailable && account.permissions?.clientUpload);
  const selectedAccount = accounts.find((account) => account.id === selectedAccountId);
  if (!can("write")) return null;
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div className="space-y-1.5"><Label htmlFor="upload-account">目标账户</Label><select id="upload-account" value={selectedAccountId} onChange={(event) => setSelectedAccountId(event.target.value)} disabled={isLoadingAccounts} className="h-9 w-full rounded-md border bg-background px-3 text-sm"><option value="">自动选择（智能分配）</option>{availableAccounts.map((account) => <option key={account.id} value={account.id}>{account.name}</option>)}</select><p className="text-xs text-muted-foreground">仅显示已启用前端上传权限的账户</p></div>
        <div className="space-y-1.5"><Label htmlFor="upload-path">上传目录</Label><div className="relative"><FolderOpen className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" /><Input id="upload-path" value={customPath} onChange={(event) => setCustomPath(event.target.value)} placeholder="留空使用日期目录" className="h-9 pl-9" /></div><p className="text-xs text-muted-foreground">例如 images/avatar 或 docs/2026</p></div>
        <div className="space-y-1.5"><Label htmlFor="upload-expiration">文件有效期</Label><div className="flex gap-2"><select id="upload-expiration" value={expirationDays} onChange={(event) => setExpirationDays(Number(event.target.value))} className="h-9 min-w-0 flex-1 rounded-md border bg-background px-3 text-sm">{EXPIRATION_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>{expirationDays === -2 && <Input aria-label="自定义有效期天数" type="number" min={1} max={3650} value={customExpirationDays} onChange={(event) => setCustomExpirationDays(event.target.value)} className="h-9 w-24" />}</div><p className="text-xs text-muted-foreground">文件到期后自动删除</p></div>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-3 border-y py-3">
        <label className="flex cursor-pointer items-center gap-2 text-sm"><Checkbox checked={!randomFilename} onCheckedChange={(checked) => setRandomFilename(checked !== true)} />保留原文件名</label><p className="text-xs text-muted-foreground">{randomFilename ? "使用随机文件名避免冲突" : "使用原文件名；重名时拒绝上传，不覆盖已有文件"}</p>
      </div>
      {selectedAccount && <p className="text-xs text-muted-foreground">容量剩余 {formatBytes(Math.max(0, selectedAccount.quota.maxSizeBytes - selectedAccount.usage.sizeBytes))} · 写请求剩余 {Math.max(0, selectedAccount.quota.maxClassAOps - selectedAccount.usage.classAOps).toLocaleString()}{selectedAccount.quota.maxClassBOps ? " · 读请求剩余 " + Math.max(0, selectedAccount.quota.maxClassBOps - selectedAccount.usage.classBOps).toLocaleString() : ""}。实际可用额度由服务器按并发任务预留校验。</p>}
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground"><span>最多 3 个文件同时上传，单文件最大 5 GiB。文件通过服务器上传。</span>{uploads.some((item) => item.status === "success") && <Button variant="outline" size="sm" onClick={() => updateItems(uploadsRef.current.filter((item) => item.status !== "success"))}>清除已完成</Button>}</div>
      <div onDragEnter={handleDragEnter} onDragOver={handleDragOver} onDragLeave={handleDragLeave} onDrop={handleDrop} onClick={() => fileInputRef.current?.click()} className={cn("cursor-pointer rounded-lg border-2 border-dashed p-8 text-center transition-colors", isDragging ? "border-primary bg-primary/5" : "border-muted-foreground/25 hover:bg-muted/50")}>
        <input ref={fileInputRef} aria-label="选择上传文件" type="file" multiple className="hidden" onChange={handleFileSelect} /><div className="flex flex-col items-center gap-3"><CloudUpload className="h-10 w-10 text-muted-foreground" /><div><p className="font-medium">{isDragging ? "释放以上传文件" : "拖拽文件到这里上传"}</p><p className="mt-1 text-sm text-muted-foreground">或点击选择文件，支持多文件上传</p></div></div>
      </div>
      {uploads.length > 0 && <div className="space-y-3">{uploads.map((upload) => <UploadItemCard key={upload.id} upload={upload} onRemove={() => removeUpload(upload.id)} onCancel={() => cancelUpload(upload.id)} onRetry={() => retryUpload(upload.id)} />)}</div>}
    </div>
  );
}

interface UploadItemCardProps {
  upload: UploadItem;
  onRemove: () => void;
  onCancel: () => void;
  onRetry: () => void;
}

function UploadItemCard({ upload, onRemove, onCancel, onRetry }: UploadItemCardProps) {
  const [copied, setCopied] = useState(false);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);

  // 生成本地预览
  useEffect(() => {
    let active = true;
    if (isImageFile(upload.file) || isVideoFile(upload.file)) {
      const url = URL.createObjectURL(upload.file);
      void Promise.resolve().then(() => { if (active) setPreviewUrl(url); });
      return () => { active = false; URL.revokeObjectURL(url); };
    }
  }, [upload.file]);

  const handleCopy = async () => {
    if (upload.result?.url) {
      await navigator.clipboard.writeText(upload.result.url);
      setCopied(true);
      toast.success("链接已复制");
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleOpenUrl = () => {
    if (upload.result?.url) {
      window.open(upload.result.url, "_blank");
    }
  };

  const isImage = isImageFile(upload.file);
  const isVideo = isVideoFile(upload.file);

  return (
    <Card className="overflow-hidden">
      <CardContent className="p-3 sm:p-4">
        <div className="flex gap-3">
          {/* 预览/图标 */}
          <div className="flex-shrink-0 w-16 h-16 sm:w-20 sm:h-20 rounded-lg overflow-hidden bg-muted flex items-center justify-center">
            {previewUrl && isImage ? (
              <img
                src={previewUrl}
                alt={upload.file.name}
                className="w-full h-full object-cover"
              />
            ) : previewUrl && isVideo ? (
              <video
                src={previewUrl}
                className="w-full h-full object-cover"
                muted
              />
            ) : isImage ? (
              <FileImage className="h-8 w-8 text-muted-foreground" />
            ) : isVideo ? (
              <FileVideo className="h-8 w-8 text-muted-foreground" />
            ) : (
              <FileIcon className="h-8 w-8 text-muted-foreground" />
            )}
          </div>

          {/* 信息 */}
          <div className="flex-1 min-w-0">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <p className="font-medium text-sm truncate">{upload.file.name}</p>
                <p className="text-xs text-muted-foreground">
                  {formatBytes(upload.file.size)}
                </p>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 flex-shrink-0"
                onClick={onRemove}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>

            {/* 状态和进度 */}
            {upload.status === "uploading" && (
              <div className="mt-2">
                <Progress value={upload.progress} className="h-1.5" />
                <div className="mt-1 flex items-center justify-between gap-2"><p className="text-xs text-muted-foreground">{upload.progress >= 99 ? "服务器处理中…" : "上传中 " + upload.progress + "%"}</p><Button variant="ghost" size="sm" className="h-7 text-xs" onClick={onCancel}>取消</Button></div>
              </div>
            )}

            {upload.status === "pending" && <p className="mt-2 text-xs text-muted-foreground">等待上传</p>}
            {(upload.status === "error" || upload.status === "cancelled") && (
              <div className="mt-2 space-y-2"><p className="text-xs text-destructive">{upload.error}</p>{upload.file.size <= MAX_FILE_BYTES && !upload.retryExpired && <Button variant="outline" size="sm" className="h-7 text-xs" onClick={onRetry}><RotateCcw className="mr-1 h-3 w-3" />重试</Button>}</div>
            )}

            {/* 成功结果 - 所有文件类型都显示 URL 和账户信息 */}
            {upload.status === "success" && upload.result && (
              <div className="mt-2 space-y-2">
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Check className="h-3.5 w-3.5 text-green-500" />
                  <span>已上传至</span>
                  <Badge variant="secondary" className="text-xs">
                    {upload.result.accountName}
                  </Badge>
                </div>
                {upload.result.url && (
                  <>
                    <div className="p-2 bg-muted rounded text-xs font-mono break-all leading-relaxed">
                      {upload.result.url}
                    </div>
                    <div className="flex gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-xs"
                        onClick={handleCopy}
                      >
                        {copied ? (
                          <Check className="h-3 w-3 mr-1 text-green-500" />
                        ) : (
                          <Copy className="h-3 w-3 mr-1" />
                        )}
                        复制链接
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-xs"
                        onClick={handleOpenUrl}
                      >
                        <ExternalLink className="h-3 w-3 mr-1" />
                        打开
                      </Button>
                    </div>
                  </>
                )}
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
