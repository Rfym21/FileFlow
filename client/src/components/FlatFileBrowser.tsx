import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Eye, Link, Trash2, CalendarClock, RefreshCw, ChevronRight } from "lucide-react";
import { useAuth } from "@/hooks/useAuth";
import { getFiles, getFileLink, batchDeleteFiles, setFilesExpiration, type FileNode, type AccountFiles } from "@/lib/api";
import { formatBytes, formatDate } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Pagination } from "@/components/ui/pagination";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import ConfirmationDialog from "@/components/ConfirmationDialog";
import FilePreviewDialog from "@/components/FilePreviewDialog";

const PAGE_SIZE = 50;
const BATCH_LIMIT = 100;
function localDateTime(date: Date): string {
  const pad = (value: number) => String(value).padStart(2, "0");
  return date.getFullYear() + "-" + pad(date.getMonth() + 1) + "-" + pad(date.getDate()) + "T" + pad(date.getHours()) + ":" + pad(date.getMinutes());
}

interface FlatFileBrowserProps {
  accountId: string;
  prefix: string;
  query: string;
  refreshKey: number;
  onAccountInfo: (info: Pick<AccountFiles, "accountName" | "sizeBytes" | "maxSize">) => void;
  onBusyChange?: (busy: boolean) => void;
}

export default function FlatFileBrowser({ accountId, prefix, query, refreshKey, onAccountInfo, onBusyChange }: FlatFileBrowserProps) {
  const { can } = useAuth();
  const canSelect = can("write") || can("delete");
  const [files, setFiles] = useState<FileNode[]>([]);
  const [nextCursor, setNextCursor] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [page, setPage] = useState(1);
  const [preview, setPreview] = useState<FileNode | null>(null);
  const [deleteTargets, setDeleteTargets] = useState<string[] | null>(null);
  const [deleteError, setDeleteError] = useState("");
  const [failures, setFailures] = useState<{ key: string; error: string }[]>([]);
  const [editingKeys, setEditingKeys] = useState<string[] | null>(null);
  const [expiresInput, setExpiresInput] = useState("");
  const [permanent, setPermanent] = useState(false);
  const [editError, setEditError] = useState("");
  const [busy, setBusy] = useState(false);
  const requestSequence = useRef(0);
  const requestController = useRef<AbortController | null>(null);
  const mutationInFlight = useRef(false);
  const accountInfoCallback = useRef(onAccountInfo);
  const currentScope = useRef({ accountId, prefix, query });
  const lastRefreshKey = useRef(refreshKey);
  const mounted = useRef(true);
  useEffect(() => { accountInfoCallback.current = onAccountInfo; }, [onAccountInfo]);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => { onBusyChange?.(busy); return () => onBusyChange?.(false); }, [busy, onBusyChange]);

  const load = useCallback(async (cursor?: string) => {
    const sequence = ++requestSequence.current;
    requestController.current?.abort();
    const controller = new AbortController();
    requestController.current = controller;
    if (cursor) setLoadingMore(true); else setLoading(true);
    setLoadError("");
    try {
      const response = await getFiles(prefix, cursor, PAGE_SIZE, [accountId], { flat: true, query, signal: controller.signal });
      if (sequence !== requestSequence.current) return;
      const data = response[0];
      const items = (data?.files || []).filter((file) => !file.isDir);
      setFiles((previous) => {
        const result = new Map((cursor ? previous : []).map((file) => [file.key, file]));
        items.forEach((file) => result.set(file.key, file));
        return Array.from(result.values());
      });
      setNextCursor(data?.nextCursor);
      if (data) accountInfoCallback.current(data);
    } catch (error) {
      if (sequence !== requestSequence.current || controller.signal.aborted) return;
      setLoadError(error instanceof Error ? error.message : "加载文件失败，请重试");
    } finally {
      if (sequence === requestSequence.current) { setLoading(false); setLoadingMore(false); }
    }
  }, [accountId, prefix, query]);

  useEffect(() => {
    let active = true;
    currentScope.current = { accountId, prefix, query };
    void Promise.resolve().then(() => { if (!active) return; setFiles([]); setNextCursor(undefined); setSelected(new Set()); setPage(1); setFailures([]); void load(); });
    return () => { active = false; requestSequence.current += 1; requestController.current?.abort(); };
  }, [load, accountId, prefix, query]);
  useEffect(() => {
    if (refreshKey === lastRefreshKey.current) return;
    lastRefreshKey.current = refreshKey;
    let active = true;
    void Promise.resolve().then(() => { if (active) void load(); });
    return () => { active = false; };
  }, [refreshKey, load]);

  const totalPages = Math.max(1, Math.ceil(files.length / PAGE_SIZE));
  const visiblePage = Math.min(page, totalPages);
  const visibleFiles = files.slice((visiblePage - 1) * PAGE_SIZE, visiblePage * PAGE_SIZE);
  const allPageSelected = visibleFiles.length > 0 && visibleFiles.every((file) => selected.has(file.key));
  const controlsBusy = busy || loading || loadingMore;
  const addSelection = (keys: string[]) => {
    setSelected((previous) => {
      const result = new Set(previous);
      for (const key of keys) { if (result.size >= BATCH_LIMIT && !result.has(key)) break; result.add(key); }
      return result;
    });
    if (new Set([...selected, ...keys]).size > BATCH_LIMIT) toast.info("每次最多选择 100 个文件；已选择可用的前 100 项");
  };
  const togglePage = () => {
    if (allPageSelected) setSelected((previous) => new Set([...previous].filter((key) => !visibleFiles.some((file) => file.key === key))));
    else addSelection(visibleFiles.map((file) => file.key));
  };
  const toggleFile = (key: string) => {
    if (selected.has(key)) setSelected((previous) => { const result = new Set(previous); result.delete(key); return result; });
    else addSelection([key]);
  };
  const beginExpiry = (keys: string[]) => {
    if (!can("write") || controlsBusy || keys.length === 0) return;
    setExpiresInput(localDateTime(new Date(Date.now() + 7 * 86400000)));
    setPermanent(false); setEditError(""); setEditingKeys(keys);
  };
  const copyLink = async (key: string) => {
    try { const result = await getFileLink(accountId, key); await navigator.clipboard.writeText(result.url); toast.success("链接已复制"); }
    catch (error) { toast.error(error instanceof Error ? error.message : "复制链接失败"); }
  };
  const removeFiles = async () => {
    if (!can("delete") || !deleteTargets || mutationInFlight.current) return;
    mutationInFlight.current = true; setBusy(true); setDeleteError("");
    const scope = currentScope.current;
    try {
      const result = await batchDeleteFiles(accountId, deleteTargets);
      if (!mounted.current || scope !== currentScope.current) return;
      const removed = new Set(result.results.filter((item) => item.success).map((item) => item.key));
      const rejected = result.results.filter((item) => !item.success).map((item) => ({ key: item.key, error: item.error || "删除失败" }));
      setFiles((previous) => previous.filter((file) => !removed.has(file.key)));
      setSelected(new Set(rejected.map((item) => item.key)));
      setFailures(rejected);
      setDeleteTargets(null);
      if (result.successCount > 0) toast.success("已删除 " + result.successCount + " 个文件");
      if (result.failCount > 0) toast.error(result.failCount + " 个文件删除失败，已保留选择以便重试");
    } catch (error) { if (mounted.current) setDeleteError(error instanceof Error ? error.message : "删除失败，请重试"); }
    finally { mutationInFlight.current = false; if (mounted.current) setBusy(false); }
  };
  const updateExpiration = async (event: FormEvent) => {
    event.preventDefault();
    if (!can("write") || !editingKeys || mutationInFlight.current) return;
    const date = new Date(expiresInput);
    if (!permanent && (!expiresInput || Number.isNaN(date.getTime()) || date.getTime() <= Date.now() || localDateTime(date) !== expiresInput)) { setEditError("请选择有效的未来时间"); return; }
    mutationInFlight.current = true; setBusy(true); setEditError("");
    try {
      const result = await setFilesExpiration(accountId, editingKeys, permanent ? null : date.toISOString());
      if (!mounted.current) return;
      toast.success(result.updatedCount + " 个文件已" + (permanent ? "设为永久" : "修改到期时间"));
      setEditingKeys(null); setSelected(new Set());
    } catch (error) { if (mounted.current) setEditError(error instanceof Error ? error.message : "修改到期时间失败"); }
    finally { mutationInFlight.current = false; if (mounted.current) setBusy(false); }
  };

  return <div className="space-y-4">
    <div className="flex flex-wrap items-center justify-between gap-2"><p className="text-sm text-muted-foreground">{query ? "搜索整个账户，匹配文件名或完整路径" : "列出当前目录及所有子目录中的文件"} · 已加载 {files.length} 个文件{nextCursor ? "，还有更多" : ""}</p><Button variant="outline" size="sm" disabled={controlsBusy} onClick={() => { void load(); }}><RefreshCw className="mr-2 h-4 w-4" />刷新列表</Button></div>
    {canSelect && <div className="space-y-2 border-y py-3"><div className="flex flex-wrap items-center gap-2"><span className="mr-2 text-sm">已选 {selected.size} 个文件</span><Button variant="outline" size="sm" disabled={controlsBusy || visibleFiles.length === 0} onClick={togglePage}>{allPageSelected ? "取消当前页选择" : "选择当前页"}</Button><Button variant="outline" size="sm" disabled={controlsBusy || files.length === 0} onClick={() => addSelection(files.map((file) => file.key))}>选择已加载文件</Button><Button variant="ghost" size="sm" disabled={controlsBusy || selected.size === 0} onClick={() => setSelected(new Set())}>清空选择</Button>{can("write") && <Button variant="outline" size="sm" disabled={controlsBusy || selected.size === 0} onClick={() => beginExpiry([...selected])}><CalendarClock className="mr-2 h-4 w-4" />批量修改到期时间</Button>}{can("delete") && <Button variant="destructive" size="sm" disabled={controlsBusy || selected.size === 0} onClick={() => { setDeleteError(""); setDeleteTargets([...selected]); }}><Trash2 className="mr-2 h-4 w-4" />批量删除</Button>}</div><p className="text-xs text-muted-foreground">选择跨已加载分页保留，每次最多 100 个文件；“选择已加载文件”不包含尚未读取的文件。</p></div>}
    {loadError && <p role="alert" className="text-sm text-destructive">{loadError}</p>}
    {failures.length > 0 && <div role="status" className="rounded-md border border-destructive/30 p-3 text-sm"><p className="font-medium">以下文件删除失败</p><ul className="mt-2 space-y-1">{failures.map((failure) => <li key={failure.key} className="break-all">{failure.key}：{failure.error}</li>)}</ul></div>}
    <div className="overflow-x-auto border-y" aria-busy={loading}>
      <table className="w-full min-w-[740px] text-left text-sm"><caption className="sr-only">扁平化文件列表</caption><thead className="border-b bg-muted/40 text-muted-foreground"><tr>{canSelect && <th scope="col" className="w-10 px-3 py-3"><Checkbox aria-label="选择当前页文件" checked={allPageSelected} disabled={controlsBusy || visibleFiles.length === 0} onCheckedChange={togglePage} /></th>}<th scope="col" className="px-3 py-3">文件路径</th><th scope="col" className="px-3 py-3">大小</th><th scope="col" className="px-3 py-3">修改时间</th><th scope="col" className="px-3 py-3 text-right">操作</th></tr></thead><tbody className="divide-y">{visibleFiles.map((file) => <tr key={file.key} className="hover:bg-muted/30">{canSelect && <td className="px-3 py-3"><Checkbox aria-label={"选择文件 " + file.key} checked={selected.has(file.key)} disabled={controlsBusy} onCheckedChange={() => toggleFile(file.key)} /></td>}<td className="max-w-md break-all px-3 py-3"><button className="text-left hover:underline" onClick={() => setPreview(file)}>{file.key}</button></td><td className="whitespace-nowrap px-3 py-3">{formatBytes(file.size || 0)}</td><td className="whitespace-nowrap px-3 py-3 text-xs text-muted-foreground">{file.lastModified ? formatDate(file.lastModified) : "—"}</td><td className="px-3 py-3"><div className="flex justify-end gap-1"><Button variant="ghost" size="icon" aria-label={"预览文件 " + file.key} onClick={() => setPreview(file)}><Eye className="h-4 w-4" /></Button><Button variant="ghost" size="icon" aria-label={"复制文件链接 " + file.key} onClick={() => { void copyLink(file.key); }}><Link className="h-4 w-4" /></Button>{can("write") && <Button variant="ghost" size="icon" aria-label={"修改文件到期时间 " + file.key} disabled={controlsBusy} onClick={() => beginExpiry([file.key])}><CalendarClock className="h-4 w-4" /></Button>}{can("delete") && <Button variant="ghost" size="icon" aria-label={"删除文件 " + file.key} className="text-destructive" disabled={controlsBusy} onClick={() => { setDeleteError(""); setDeleteTargets([file.key]); }}><Trash2 className="h-4 w-4" /></Button>}</div></td></tr>)}{visibleFiles.length === 0 && <tr><td colSpan={canSelect ? 5 : 4} className="py-12 text-center text-muted-foreground">{loading ? "正在读取文件…" : nextCursor ? "当前批次没有匹配文件，可继续读取下一批" : query ? "没有找到匹配文件" : "此目录下没有文件"}</td></tr>}</tbody></table>
    </div>
    <fieldset disabled={controlsBusy}><Pagination currentPage={visiblePage} totalPages={totalPages} onPageChange={setPage} totalItems={files.length} pageSize={PAGE_SIZE} /></fieldset>
    {nextCursor && <div className="flex justify-center"><Button variant="outline" disabled={controlsBusy} onClick={() => { void load(nextCursor); }}>{loadingMore ? <RefreshCw className="mr-2 h-4 w-4 animate-spin" /> : <ChevronRight className="mr-2 h-4 w-4" />}{loadingMore ? "读取中…" : query ? "继续搜索下一批" : "加载更多文件"}</Button></div>}
    <FilePreviewDialog accountId={accountId} file={preview} onClose={() => setPreview(null)} />
    <ConfirmationDialog open={deleteTargets !== null && can("delete")} onOpenChange={(open) => { if (!open && !mutationInFlight.current) { setDeleteTargets(null); setDeleteError(""); } }} title="永久删除所选文件？" description={"将从 R2 永久删除 " + (deleteTargets?.length || 0) + " 个文件，此操作无法恢复。"} confirmLabel="永久删除文件" busy={busy} error={deleteError} onConfirm={removeFiles}><div className="max-h-48 overflow-y-auto rounded-md bg-muted p-3 text-xs">{deleteTargets?.map((key) => <p className="break-all py-1" key={key}>{key}</p>)}</div></ConfirmationDialog>
    <Dialog open={editingKeys !== null && can("write")} onOpenChange={(open) => { if (!open && !mutationInFlight.current) setEditingKeys(null); }}><DialogContent onEscapeKeyDown={(event) => { if (busy) event.preventDefault(); }} onPointerDownOutside={(event) => { if (busy) event.preventDefault(); }}><DialogHeader><DialogTitle>修改文件到期时间</DialogTitle><DialogDescription>为 {editingKeys?.length || 0} 个文件设置到期时间，到期后会自动删除。</DialogDescription></DialogHeader><form className="space-y-4" onSubmit={(event) => { void updateExpiration(event); }}><fieldset disabled={busy} className="space-y-4"><label className="flex items-center gap-2 text-sm"><Checkbox checked={permanent} onCheckedChange={(checked) => setPermanent(checked === true)} />永久保存</label>{!permanent && <div className="space-y-2"><Label htmlFor="flat-file-expiration">到期时间</Label><Input id="flat-file-expiration" type="datetime-local" value={expiresInput} onChange={(event) => setExpiresInput(event.target.value)} required /><p className="text-xs text-muted-foreground">使用本地时区 {Intl.DateTimeFormat().resolvedOptions().timeZone}</p></div>}</fieldset>{editError && <p role="alert" className="text-sm text-destructive">{editError}</p>}<DialogFooter><Button variant="outline" type="button" disabled={busy} onClick={() => setEditingKeys(null)}>取消</Button><Button type="submit" disabled={busy}>{busy ? "保存中…" : "保存"}</Button></DialogFooter></form></DialogContent></Dialog>
  </div>;
}
