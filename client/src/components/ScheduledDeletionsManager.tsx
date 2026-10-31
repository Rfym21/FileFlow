import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { Button } from "@/components/ui/button";
import { Pagination } from "@/components/ui/pagination";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  getFileExpirations,
  deleteFileExpirations,
  updateFileExpiration,
  updateFileExpirations,
  type FileExpiration,
} from "@/lib/api";
import { formatDate } from "@/lib/utils";
import { useAuth } from "@/hooks/useAuth";
import {
  Trash2,
  RefreshCw,
  Clock,
  CalendarClock,
  AlertTriangle,
  Search,
  X,
} from "lucide-react";
import { toast } from "sonner";

const PAGE_SIZE = 10;

function getTimeRemaining(expiresAt: string): {
  text: string;
  isExpired: boolean;
  isUrgent: boolean;
} {
  const diff = new Date(expiresAt).getTime() - Date.now();
  if (diff <= 0) return { text: "已过期", isExpired: true, isUrgent: false };
  const days = Math.floor(diff / 86400000);
  const hours = Math.floor((diff % 86400000) / 3600000);
  if (days > 0)
    return {
      text: `${days} 天 ${hours} 小时`,
      isExpired: false,
      isUrgent: days <= 3,
    };
  if (hours > 0)
    return { text: `${hours} 小时`, isExpired: false, isUrgent: true };
  return {
    text: `${Math.floor(diff / 60000)} 分钟`,
    isExpired: false,
    isUrgent: true,
  };
}

function sortExpirations(records: FileExpiration[]): FileExpiration[] {
  return [...records].sort(
    (a, b) => new Date(a.expiresAt).getTime() - new Date(b.expiresAt).getTime(),
  );
}

// datetime-local uses local time; the API accepts an ISO timestamp.
function toLocalDateTime(value: string): string {
  const date = new Date(value);
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

interface EditTarget {
  ids: string[];
  records: FileExpiration[];
  hiddenCount: number;
  fileKey?: string;
  originalExpiresAt?: string;
  originalLocalInput?: string;
}

function resolveEditedExpiration(input: string, target: EditTarget): Date {
  if (target.originalExpiresAt && input === target.originalLocalInput) {
    return new Date(target.originalExpiresAt);
  }
  const parsed = new Date(input);
  if (!target.originalExpiresAt || Number.isNaN(parsed.getTime()))
    return parsed;

  // A repeated local hour can represent two instants. Prefer the original
  // offset when it still represents the exact local time the user entered.
  const originalOffset = new Date(target.originalExpiresAt).getTimezoneOffset();
  const preferred = new Date(
    parsed.getTime() + (originalOffset - parsed.getTimezoneOffset()) * 60000,
  );
  if (
    !Number.isNaN(preferred.getTime()) &&
    preferred.getTimezoneOffset() === originalOffset &&
    toLocalDateTime(preferred.toISOString()) ===
      toLocalDateTime(parsed.toISOString())
  )
    return preferred;
  return parsed;
}

interface DeleteFailure {
  id: string;
  fileKey: string;
  error: string;
}

export default function ScheduledDeletionsManager() {
  const { can } = useAuth();
  const canModify = can("write") || can("delete");
  const [expirations, setExpirations] = useState<FileExpiration[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [mutation, setMutation] = useState<"update" | "delete" | null>(null);
  const mutationInFlight = useRef(false);
  const latestLoadRequest = useRef(0);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [searchQuery, setSearchQuery] = useState("");
  const [currentPage, setCurrentPage] = useState(1);
  const [editTarget, setEditTarget] = useState<EditTarget | null>(null);
  const [expiresAtInput, setExpiresAtInput] = useState("");
  const [editError, setEditError] = useState("");
  const [deleteTargets, setDeleteTargets] = useState<FileExpiration[] | null>(
    null,
  );
  const [deleteError, setDeleteError] = useState("");
  const [deleteHiddenCount, setDeleteHiddenCount] = useState(0);
  const [deleteFailures, setDeleteFailures] = useState<DeleteFailure[]>([]);
  const busy = loading || mutation !== null;
  const localTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const normalizedQuery = searchQuery.trim().toLocaleLowerCase();
  const filteredExpirations = useMemo(
    () =>
      normalizedQuery
        ? expirations.filter((exp) =>
            [exp.fileKey, exp.accountName || "", exp.accountId].some((value) =>
              value.toLocaleLowerCase().includes(normalizedQuery),
            ),
          )
        : expirations,
    [expirations, normalizedQuery],
  );
  const filteredIds = useMemo(
    () => new Set(filteredExpirations.map((exp) => exp.id)),
    [filteredExpirations],
  );
  const totalPages = Math.max(
    1,
    Math.ceil(filteredExpirations.length / PAGE_SIZE),
  );
  const visiblePage = Math.min(currentPage, totalPages);
  const paginatedExpirations = useMemo(
    () =>
      filteredExpirations.slice(
        (visiblePage - 1) * PAGE_SIZE,
        visiblePage * PAGE_SIZE,
      ),
    [filteredExpirations, visiblePage],
  );
  const selectedExpirations = useMemo(
    () => expirations.filter((exp) => selectedIds.has(exp.id)),
    [expirations, selectedIds],
  );
  const visibleSelectedCount = selectedExpirations.filter((exp) =>
    filteredIds.has(exp.id),
  ).length;
  const hiddenSelectedCount = selectedExpirations.length - visibleSelectedCount;
  const pageSelectedCount = paginatedExpirations.filter((exp) =>
    selectedIds.has(exp.id),
  ).length;
  const pageChecked =
    pageSelectedCount === paginatedExpirations.length && pageSelectedCount > 0
      ? true
      : pageSelectedCount > 0
        ? "indeterminate"
        : false;

  const loadExpirations = useCallback(async () => {
    const requestId = ++latestLoadRequest.current;
    try {
      const data = await getFileExpirations();
      if (requestId !== latestLoadRequest.current) return;
      const sorted = sortExpirations(data.expirations || []);
      const validIds = new Set(sorted.map((exp) => exp.id));
      setExpirations(sorted);
      setSelectedIds(
        (previous) => new Set([...previous].filter((id) => validIds.has(id))),
      );
      setDeleteFailures((previous) =>
        previous.filter((failure) => validIds.has(failure.id)),
      );
      setLoadError("");
    } catch (err) {
      if (requestId !== latestLoadRequest.current) return;
      const message = err instanceof Error ? err.message : "加载到期列表失败";
      setLoadError(message);
      toast.error(message);
    } finally {
      if (requestId === latestLoadRequest.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadExpirations();
  }, [loadExpirations]);

  const refresh = () => {
    if (busy) return;
    setLoading(true);
    void loadExpirations();
  };

  const selectPage = (checked: boolean) => {
    setSelectedIds((previous) => {
      const next = new Set(previous);
      paginatedExpirations.forEach((exp) => {
        if (checked) next.add(exp.id);
        else next.delete(exp.id);
      });
      return next;
    });
  };

  const changeSearch = (query: string) => {
    setSearchQuery(query);
    setCurrentPage(1);
  };

  const selectSearchResults = () => {
    setSelectedIds((previous) => {
      const next = new Set(previous);
      filteredExpirations.forEach((exp) => next.add(exp.id));
      return next;
    });
  };

  const openEdit = (records: FileExpiration[]) => {
    if (!can("write") || busy || records.length === 0) return;
    const originalExpiresAt =
      records.length === 1 ? records[0].expiresAt : undefined;
    const localInput = toLocalDateTime(
      originalExpiresAt || new Date(Date.now() + 86400000).toISOString(),
    );
    setEditTarget({
      ids: records.map((exp) => exp.id),
      records,
      hiddenCount: records.filter((exp) => !filteredIds.has(exp.id)).length,
      fileKey: records.length === 1 ? records[0].fileKey : undefined,
      originalExpiresAt,
      originalLocalInput: originalExpiresAt ? localInput : undefined,
    });
    setExpiresAtInput(localInput);
    setEditError("");
  };

  const saveExpiration = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!can("write")) return;
    if (!editTarget || mutationInFlight.current) return;
    const date = resolveEditedExpiration(expiresAtInput, editTarget);
    if (!expiresAtInput || Number.isNaN(date.getTime())) {
      setEditError("请选择有效的到期时间");
      return;
    }
    if (date.getTime() <= Date.now()) {
      setEditError("到期时间必须晚于当前时间");
      return;
    }
    mutationInFlight.current = true;
    setMutation("update");
    setEditError("");
    const expiresAt =
      expiresAtInput === editTarget.originalLocalInput
        ? editTarget.originalExpiresAt || date.toISOString()
        : date.toISOString();
    try {
      const result =
        editTarget.ids.length === 1
          ? await updateFileExpiration(editTarget.ids[0], expiresAt)
          : await updateFileExpirations(editTarget.ids, expiresAt);
      const updatedIds = new Set(editTarget.ids);
      setExpirations((previous) =>
        sortExpirations(
          previous.map((exp) =>
            updatedIds.has(exp.id) ? { ...exp, expiresAt } : exp,
          ),
        ),
      );
      setEditTarget(null);
      toast.success(`已修改 ${result.updatedCount} 个文件的到期时间`);
    } catch (err) {
      setEditError(err instanceof Error ? err.message : "修改到期时间失败");
    } finally {
      mutationInFlight.current = false;
      setMutation(null);
    }
  };

  const openDelete = (records: FileExpiration[]) => {
    if (!can("delete")) return;
    if (busy || records.length === 0) return;
    setDeleteTargets(records);
    setDeleteHiddenCount(records.filter((exp) => !filteredIds.has(exp.id)).length);
    setDeleteError("");
  };

  const confirmDelete = async () => {
    if (!can("delete") || !deleteTargets || mutationInFlight.current) return;
    mutationInFlight.current = true;
    setMutation("delete");
    setDeleteError("");
    try {
      const response = await deleteFileExpirations(
        deleteTargets.map((exp) => exp.id),
      );
      const results = new Map(
        response.results.map((result) => [result.id, result]),
      );
      const successfulIds = new Set(
        deleteTargets
          .filter((exp) => results.get(exp.id)?.success)
          .map((exp) => exp.id),
      );
      const failures = deleteTargets
        .filter((exp) => !successfulIds.has(exp.id))
        .map((exp) => ({
          id: exp.id,
          fileKey: exp.fileKey,
          error: results.get(exp.id)?.error || "未能确认删除成功，请刷新后重试",
        }));
      const remaining = expirations.filter((exp) => !successfulIds.has(exp.id));
      setExpirations(remaining);
      setSelectedIds((previous) => {
        const next = new Set(
          [...previous].filter((id) => !successfulIds.has(id)),
        );
        failures.forEach((failure) => next.add(failure.id));
        return next;
      });
      setDeleteFailures(failures);
      setDeleteTargets(null);
      if (successfulIds.size > 0)
        toast.success(`已永久删除 ${successfulIds.size} 个文件`);
      if (failures.length > 0)
        toast.error(`${failures.length} 个文件删除失败，已保留选中，可重试`);
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : "删除失败，请重试");
    } finally {
      mutationInFlight.current = false;
      setMutation(null);
    }
  };

  const expiredCount = expirations.filter(
    (exp) => getTimeRemaining(exp.expiresAt).isExpired,
  ).length;
  const urgentCount = expirations.filter((exp) => {
    const status = getTimeRemaining(exp.expiresAt);
    return !status.isExpired && status.isUrgent;
  }).length;

  return (
    <div className="space-y-4" aria-busy={busy}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
          <span>共 {expirations.length} 个计划删除文件</span>
          {expiredCount > 0 && (
            <Badge variant="destructive">{expiredCount} 个已过期</Badge>
          )}
          {urgentCount > 0 && (
            <Badge
              variant="outline"
              className="border-yellow-500 text-yellow-600"
            >
              {urgentCount} 个即将删除
            </Badge>
          )}
        </div>
        <Button variant="outline" onClick={refresh} disabled={busy}>
          <RefreshCw
            className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`}
            aria-hidden="true"
          />
          {loading ? "加载中" : "刷新"}
        </Button>
      </div>
      <div className="space-y-2">
        <Label htmlFor="scheduled-search">搜索计划删除文件</Label>
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative w-full sm:max-w-md">
            <Search
              className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              id="scheduled-search"
              type="search"
              value={searchQuery}
              onChange={(event) => changeSearch(event.target.value)}
              placeholder="搜索文件路径或账户名称"
              className="pl-9"
              disabled={mutation !== null}
              aria-describedby="scheduled-search-results"
            />
          </div>
          {searchQuery && (
            <Button
              variant="outline"
              onClick={() => changeSearch("")}
              disabled={mutation !== null}
            >
              <X className="mr-2 h-4 w-4" aria-hidden="true" />
              清空搜索
            </Button>
          )}
          <p
            id="scheduled-search-results"
            className="text-sm text-muted-foreground"
            aria-live="polite"
          >
            {normalizedQuery
              ? `找到 ${filteredExpirations.length} / ${expirations.length} 个文件`
              : `显示全部 ${expirations.length} 个文件`}
          </p>
        </div>
      </div>
      {loadError && (
        <div
          role="alert"
          className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm"
        >
          <span className="text-destructive">加载失败：{loadError}</span>
          <Button variant="outline" size="sm" onClick={refresh} disabled={busy}>
            重试加载
          </Button>
        </div>
      )}
      {deleteFailures.length > 0 && (
        <div
          role="alert"
          className="space-y-2 rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm"
        >
          <div className="flex items-center justify-between gap-3">
            <p className="font-medium text-destructive">
              {deleteFailures.length} 个文件删除失败，请查看错误并重试
            </p>
            <Button
              variant="ghost"
              size="icon"
              aria-label="关闭删除错误提示"
              onClick={() => setDeleteFailures([])}
              disabled={busy}
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
          <ul className="max-h-48 space-y-1 overflow-y-auto">
            {deleteFailures.map((failure) => (
              <li key={failure.id} className="break-all">
                <span className="font-medium">{failure.fileKey}</span>：
                {failure.error}
              </li>
            ))}
          </ul>
        </div>
      )}
      {canModify && expirations.length > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-muted/30 px-4 py-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="mr-2 text-sm font-medium" aria-live="polite">
              已选总计 {selectedExpirations.length} 个，当前搜索结果中{" "}
              {visibleSelectedCount} 个
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => selectPage(true)}
              disabled={busy || paginatedExpirations.length === 0}
            >
              选择当前页（{paginatedExpirations.length}）
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={selectSearchResults}
              disabled={busy || filteredExpirations.length === 0}
            >
              选择全部搜索结果（{filteredExpirations.length}）
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelectedIds(new Set())}
              disabled={busy || selectedExpirations.length === 0}
            >
              清空选择
            </Button>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {can("write") && <Button
              variant="outline"
              size="sm"
              onClick={() => openEdit(selectedExpirations)}
              disabled={busy || selectedIds.size === 0}
            >
              <CalendarClock className="mr-2 h-4 w-4" aria-hidden="true" />
              批量修改到期时间（全部已选 {selectedExpirations.length}）
            </Button>}
            {can("delete") && <Button
              variant="destructive"
              size="sm"
              onClick={() => openDelete(selectedExpirations)}
              disabled={busy || selectedIds.size === 0}
            >
              <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
              批量删除全部已选（{selectedExpirations.length}）
            </Button>}
          </div>
          <p className="w-full text-xs text-muted-foreground">
            选择会跨页、跨搜索保留；“选择全部搜索结果”仅新增当前搜索匹配的文件。批量操作会处理全部已选文件。
          </p>
          {hiddenSelectedCount > 0 && (
            <p className="w-full text-sm text-yellow-600" role="status">
              {hiddenSelectedCount} 个已选文件不在当前搜索结果中，批量修改或删除也会包含这些文件。
            </p>
          )}
        </div>
      )}
      {loading && expirations.length === 0 ? (
        <div
          role="status"
          className="flex h-64 items-center justify-center gap-3 text-muted-foreground"
        >
          <RefreshCw className="h-8 w-8 animate-spin" aria-hidden="true" />
          <span>加载计划删除列表…</span>
        </div>
      ) : expirations.length === 0 ? (
        !loadError && (
          <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
            <Clock className="mb-4 h-12 w-12 opacity-50" aria-hidden="true" />
            <p>暂无计划删除的文件</p>
            <p className="text-sm">上传时设置有效期的文件会出现在这里</p>
          </div>
        )
      ) : filteredExpirations.length === 0 ? (
        <div className="flex flex-col items-center justify-center gap-3 py-12 text-muted-foreground">
          <Search className="h-10 w-10 opacity-50" aria-hidden="true" />
          <p>没有匹配的计划删除文件</p>
          <Button variant="outline" onClick={() => changeSearch("")} disabled={mutation !== null}>
            清空搜索
          </Button>
        </div>
      ) : (
        <>
          <div className="overflow-x-auto rounded-lg border">
            <table className="w-full min-w-[1080px] text-sm">
              <caption className="sr-only">
                计划删除文件，按到期时间从近到远排序
              </caption>
              <thead className="border-b bg-muted/50 text-left text-muted-foreground">
                <tr>
                  {canModify && <th scope="col" className="w-12 px-4 py-3">
                    <Checkbox
                      aria-label="选择或取消选择当前页所有文件"
                      checked={pageChecked}
                      onCheckedChange={(checked) =>
                        selectPage(checked === true)
                      }
                      disabled={busy}
                    />
                  </th>}
                  <th scope="col" className="min-w-64 px-4 py-3 font-medium">
                    文件路径
                  </th>
                  <th scope="col" className="px-4 py-3 font-medium">
                    账户
                  </th>
                  <th scope="col" className="px-4 py-3 font-medium">
                    创建时间
                  </th>
                  <th scope="col" className="px-4 py-3 font-medium">
                    到期时间
                  </th>
                  <th scope="col" className="px-4 py-3 font-medium">
                    剩余状态
                  </th>
                  {canModify && <th scope="col" className="px-4 py-3 font-medium">操作</th>}
                </tr>
              </thead>
              <tbody>
                {paginatedExpirations.map((exp) => {
                  const status = getTimeRemaining(exp.expiresAt);
                  return (
                    <tr
                      key={exp.id}
                      className={`border-b last:border-0 ${selectedIds.has(exp.id) ? "bg-primary/5" : "hover:bg-muted/30"}`}
                    >
                      {canModify && <td className="px-4 py-4">
                        <Checkbox
                          aria-label={`选择文件 ${exp.fileKey}`}
                          checked={selectedIds.has(exp.id)}
                          disabled={busy}
                          onCheckedChange={(checked) =>
                            setSelectedIds((previous) => {
                              const next = new Set(previous);
                              if (checked === true) next.add(exp.id);
                              else next.delete(exp.id);
                              return next;
                            })
                          }
                        />
                      </td>}
                      <td className="max-w-sm break-all px-4 py-4 font-medium">
                        {exp.fileKey}
                      </td>
                      <td className="px-4 py-4">
                        <Badge
                          variant="secondary"
                          className="max-w-40 break-all font-normal"
                        >
                          {exp.accountName || exp.accountId}
                        </Badge>
                      </td>
                      <td className="whitespace-nowrap px-4 py-4 text-xs text-muted-foreground">
                        {formatDate(exp.createdAt)}
                      </td>
                      <td className="whitespace-nowrap px-4 py-4 text-xs">
                        {formatDate(exp.expiresAt)}
                      </td>
                      <td className="whitespace-nowrap px-4 py-4">
                        <span
                          className={`inline-flex items-center gap-1 ${status.isExpired ? "text-destructive" : status.isUrgent ? "text-yellow-600" : "text-muted-foreground"}`}
                        >
                          {(status.isExpired || status.isUrgent) && (
                            <AlertTriangle
                              className="h-4 w-4"
                              aria-hidden="true"
                            />
                          )}
                          {status.text}
                        </span>
                      </td>
                      {canModify && <td className="px-4 py-4">
                        <div className="flex items-center gap-1">
                          {can("write") && <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => openEdit([exp])}
                            disabled={busy}
                            aria-label={`修改文件 ${exp.fileKey} 的到期时间`}
                          >
                            <CalendarClock
                              className="mr-1 h-4 w-4"
                              aria-hidden="true"
                            />
                            修改时间
                          </Button>}
                          {can("delete") && <Button
                            variant="ghost"
                            size="icon"
                            className="text-destructive hover:text-destructive"
                            onClick={() => openDelete([exp])}
                            disabled={busy}
                            aria-label={`永久删除文件 ${exp.fileKey}`}
                          >
                            <Trash2 className="h-4 w-4" aria-hidden="true" />
                          </Button>}
                        </div>
                      </td>}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <fieldset
            disabled={busy}
            className="min-w-0"
            aria-label="计划删除列表分页"
          >
            <Pagination
              currentPage={visiblePage}
              totalPages={totalPages}
              onPageChange={setCurrentPage}
              totalItems={filteredExpirations.length}
              pageSize={PAGE_SIZE}
            />
          </fieldset>
        </>
      )}

      <Dialog
        open={can("write") && editTarget !== null}
        onOpenChange={(open) => {
          if (!open && !mutationInFlight.current) setEditTarget(null);
        }}
      >
        <DialogContent className="max-w-[calc(100%-2rem)] sm:max-w-lg">
          <form onSubmit={saveExpiration} className="space-y-4">
            <DialogHeader>
              <DialogTitle>
                {editTarget?.ids.length === 1
                  ? "修改到期时间"
                  : "批量修改到期时间"}
              </DialogTitle>
              <DialogDescription>
                为 {editTarget?.ids.length || 0}{" "}
                个文件设置新的到期时间，到期后将自动删除。
                {editTarget && editTarget.hiddenCount > 0 &&
                  ` 其中 ${editTarget.hiddenCount} 个文件不在当前搜索结果中，也会一并修改。`}
              </DialogDescription>
            </DialogHeader>
            {editTarget?.fileKey && (
              <p className="break-all rounded-md bg-muted p-3 text-sm">
                {editTarget.fileKey}
              </p>
            )}
            {editTarget && editTarget.records.length > 1 && (
              <ul className="max-h-40 space-y-1 overflow-y-auto rounded-md bg-muted p-3 text-sm">
                {editTarget.records.map((exp) => (
                  <li key={exp.id} className="break-all">
                    {exp.fileKey}
                    <span className="ml-2 text-muted-foreground">
                      （{exp.accountName || exp.accountId}）
                    </span>
                  </li>
                ))}
              </ul>
            )}
            <div className="space-y-2">
              <Label htmlFor="scheduled-expires-at">新的到期时间</Label>
              <Input
                id="scheduled-expires-at"
                type="datetime-local"
                step="1"
                required
                value={expiresAtInput}
                onChange={(event) => {
                  setExpiresAtInput(event.target.value);
                  setEditError("");
                }}
                disabled={mutation !== null}
                aria-invalid={!!editError}
                aria-describedby="scheduled-time-help scheduled-time-error"
              />
              <p
                id="scheduled-time-help"
                className="text-xs text-muted-foreground"
              >
                按本地时区（{localTimeZone}
                ）填写，时间必须晚于现在。批量修改会将所选文件设置为同一到期时间。
              </p>
              <p
                id="scheduled-time-error"
                role={editError ? "alert" : undefined}
                className="text-sm text-destructive"
              >
                {editError}
              </p>
            </div>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setEditTarget(null)}
                disabled={mutation !== null}
              >
                取消
              </Button>
              <Button type="submit" disabled={mutation !== null}>
                {mutation === "update" && (
                  <RefreshCw
                    className="mr-2 h-4 w-4 animate-spin"
                    aria-hidden="true"
                  />
                )}
                {mutation === "update" ? "保存中…" : "保存到期时间"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <AlertDialog
        open={can("delete") && deleteTargets !== null}
        onOpenChange={(open) => {
          if (!open && !mutationInFlight.current) setDeleteTargets(null);
        }}
      >
        <AlertDialogContent className="max-w-[calc(100%-2rem)] sm:max-w-lg">
          <AlertDialogHeader>
            <AlertDialogTitle>
              永久删除 {deleteTargets?.length || 0} 个文件？
            </AlertDialogTitle>
            <AlertDialogDescription>
              将立即从存储中永久删除这 {deleteTargets?.length || 0}{" "}
              个文件，并移除对应的计划删除记录。此操作无法撤销。
              {deleteHiddenCount > 0 &&
                ` 其中 ${deleteHiddenCount} 个文件不在当前搜索结果中，也会一并删除。`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <ul className="max-h-48 space-y-1 overflow-y-auto rounded-md bg-muted p-3 text-sm">
            {deleteTargets?.map((exp) => (
              <li key={exp.id} className="break-all">
                {exp.fileKey}
                <span className="ml-2 text-muted-foreground">
                  （{exp.accountName || exp.accountId}）
                </span>
              </li>
            ))}
          </ul>
          {deleteError && (
            <p role="alert" className="text-sm text-destructive">
              {deleteError}
            </p>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={mutation !== null}>
              取消
            </AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              disabled={mutation !== null}
              onClick={(event) => {
                event.preventDefault();
                void confirmDelete();
              }}
            >
              {mutation === "delete" && (
                <RefreshCw
                  className="mr-2 h-4 w-4 animate-spin"
                  aria-hidden="true"
                />
              )}
              {mutation === "delete"
                ? "删除中…"
                : `确认删除 ${deleteTargets?.length || 0} 个文件`}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
