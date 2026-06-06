import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import {
  AlertTriangleIcon,
  CheckIcon,
  CheckCircle2Icon,
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  ChevronsLeftIcon,
  ChevronsRightIcon,
  ClipboardListIcon,
  DownloadIcon,
  EyeIcon,
  FileClockIcon,
  FilterIcon,
  SearchIcon,
  ShieldAlertIcon,
  XIcon,
} from "lucide-react";
import { fetchAlerts, fetchAuditLogs, fetchTasks, resolveAlert, type Alert, type AuditLog, type Task } from "../../lib/api";
import { formatTimeAgo } from "../../lib/format";
import { DialogPortal } from "../../components/kvm/DialogPortal";
import { ExportDialog } from "../../components/kvm/ExportDialog";
import { onKvmRefresh } from "../../lib/refresh";
import { can } from "../../lib/permissions";
import { localTimestamp, type ExportColumn } from "../../lib/exportData";

type TabKey = "tasks" | "audit" | "alerts";
type PageSize = 30 | 50 | 100 | 200 | "all";
type JsonFilter = { key: string; value: string };
type DetailItem = { type: TabKey; item: Task | AuditLog | Alert } | null;

const tabs: Array<{ key: TabKey; label: string; icon: React.ElementType }> = [
  { key: "tasks", label: "任务", icon: ClipboardListIcon },
  { key: "audit", label: "审计", icon: FileClockIcon },
  { key: "alerts", label: "告警", icon: ShieldAlertIcon },
];
const pageSizes: Array<{ value: PageSize; label: string }> = [{ value: 30, label: "30" }, { value: 50, label: "50" }, { value: 100, label: "100" }, { value: 200, label: "200" }, { value: "all", label: "全部" }];
const alertStatuses = [{ value: "", label: "全部告警" }, { value: "active", label: "活跃" }, { value: "resolved", label: "已解决" }];
const taskStatuses = [{ value: "", label: "全部任务" }, { value: "queued", label: "排队" }, { value: "running", label: "运行" }, { value: "completed", label: "完成" }, { value: "failed", label: "失败" }];
const searchPlaceholders: Record<TabKey, string> = {
  tasks: "搜索任务类型、状态、目标、进度或错误信息",
  audit: "搜索审计动作、用户、资源、IP 或元数据",
  alerts: "搜索告警级别、状态、标题、消息、来源或元数据",
};

const jsonFilterLabels: Record<TabKey, { title: string; keyPlaceholder: string; valuePlaceholder: string; sourceName: string }> = {
  tasks: { title: "任务载荷筛选", keyPlaceholder: "字段，如 vm", valuePlaceholder: "值，如 test", sourceName: "任务载荷" },
  audit: { title: "审计元数据筛选", keyPlaceholder: "字段，如 agent", valuePlaceholder: "值，如 test", sourceName: "审计元数据" },
  alerts: { title: "告警元数据筛选", keyPlaceholder: "字段，如 metric", valuePlaceholder: "值，如 disk", sourceName: "告警元数据" },
};

export default function OperationsPage() {
  const [tab, setTab] = useState<TabKey>("tasks");
  const [pageSize, setPageSize] = useState<PageSize>(50);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [query, setQuery] = useState("");
  const [jsonFilters, setJsonFilters] = useState<Record<TabKey, JsonFilter>>({
    tasks: { key: "", value: "" },
    audit: { key: "", value: "" },
    alerts: { key: "", value: "" },
  });
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [status, setStatus] = useState("");
  const [detail, setDetail] = useState<DetailItem>(null);
  const [busyId, setBusyId] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [exportOpen, setExportOpen] = useState(false);
  const [exportLoading, setExportLoading] = useState(false);
  const [exportTasks, setExportTasks] = useState<Task[]>([]);
  const [exportLogs, setExportLogs] = useState<AuditLog[]>([]);
  const [exportAlerts, setExportAlerts] = useState<Alert[]>([]);
  const canManageAlerts = can("alerts.manage");
  const currentJsonFilter = jsonFilters[tab];
  const activeJsonFilter = useMemo(() => ({
    key: currentJsonFilter.key.trim(),
    value: currentJsonFilter.value.trim(),
  }), [currentJsonFilter.key, currentJsonFilter.value]);
  const hasAdvancedFilter = activeJsonFilter.key !== "" || activeJsonFilter.value !== "";

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      if (tab === "tasks") {
        const data = await fetchTasks(pageSize, page, query, status, activeJsonFilter);
        setTasks(data.items);
        setTotal(data.total);
      } else if (tab === "audit") {
        const data = await fetchAuditLogs(pageSize, page, query, activeJsonFilter);
        setLogs(data.items);
        setTotal(data.total);
      } else {
        const data = await fetchAlerts({ status: status || undefined, limit: pageSize, page, q: query, metadata: activeJsonFilter });
        setAlerts(data.items);
        setTotal(data.total);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "读取运维记录失败";
      toast.error(message);
      setError(isPermissionMessage(message) ? "" : message);
    } finally {
      setLoading(false);
    }
  }, [activeJsonFilter, page, pageSize, query, status, tab]);

  useEffect(() => { void load(); const unsubscribe = onKvmRefresh(() => void load()); return unsubscribe; }, [load]);
  useEffect(() => { setPage(1); setStatus(""); }, [tab, pageSize]);
  useEffect(() => { setPage(1); }, [status]);
  useEffect(() => { setPage(1); }, [query]);
  useEffect(() => { setPage(1); }, [activeJsonFilter]);

  const filteredTasks = tasks;
  const filteredLogs = logs;
  const filteredAlerts = alerts;
  const effectiveTotal = total;
  const pageCount = pageSize === "all" ? 1 : Math.max(1, Math.ceil(effectiveTotal / pageSize));
  const currentPage = Math.min(page, pageCount);
  const visibleTasks = filteredTasks;
  const visibleLogs = filteredLogs;
  const visibleAlerts = filteredAlerts;
  const visibleCount = tab === "tasks" ? visibleTasks.length : tab === "audit" ? visibleLogs.length : visibleAlerts.length;
  const start = effectiveTotal === 0 ? 0 : pageSize === "all" ? 1 : (currentPage - 1) * pageSize + 1;
  const end = pageSize === "all" ? effectiveTotal : Math.min(effectiveTotal, (currentPage - 1) * pageSize + visibleCount);

  useEffect(() => { if (page > pageCount) setPage(pageCount); }, [page, pageCount]);

  const handleResolveAlert = async (alert: Alert) => {
    setBusyId(alert.id);
    try {
      await resolveAlert(alert.id);
      await load();
      toast.success("告警已解决");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "解决告警失败");
    } finally {
      setBusyId("");
    }
  };

  const openExportDialog = async () => {
    setExportTasks([]);
    setExportLogs([]);
    setExportAlerts([]);
    setExportLoading(true);
    setExportOpen(true);
    try {
      if (tab === "tasks") {
        const data = await fetchTasks("all", 1, query, status, activeJsonFilter);
        setExportTasks(data.items);
      } else if (tab === "audit") {
        const data = await fetchAuditLogs("all", 1, query, activeJsonFilter);
        setExportLogs(data.items);
      } else {
        const data = await fetchAlerts({ status: status || undefined, limit: "all", page: 1, q: query, metadata: activeJsonFilter });
        setExportAlerts(data.items);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "读取导出数据失败");
    } finally {
      setExportLoading(false);
    }
  };

  const updateJsonFilter = (field: keyof JsonFilter, value: string) => {
    setJsonFilters((current) => ({ ...current, [tab]: { ...current[tab], [field]: value } }));
  };

  const clearJsonFilter = () => {
    setJsonFilters((current) => ({ ...current, [tab]: { key: "", value: "" } }));
  };

  return (
    <div data-cmp="OperationsPage" className="space-y-5 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3"><div><h1 className="text-lg font-semibold" style={{ color: "var(--kvm-text)" }}>任务 / 审计 / 告警</h1><p className="mt-1 text-sm" style={{ color: "var(--kvm-text-muted)" }}>只读查看后台刷新、虚拟机操作、审计日志和平台告警</p></div><button type="button" onClick={() => void openExportDialog()} className="kvm-action-button flex h-10 items-center gap-2 rounded-lg border px-3 text-sm" style={{ borderColor: "rgba(59,130,246,0.38)", color: "var(--kvm-accent-text)", background: "rgba(59,130,246,0.1)" }}><DownloadIcon size={14} />导出</button></div>
      <div className="flex flex-wrap items-center justify-between gap-3"><div className="flex w-fit flex-wrap gap-2 rounded-xl p-2" style={{ background: "var(--kvm-card)", border: "1px solid var(--kvm-border)" }}>{tabs.map((item) => { const Icon = item.icon; const active = tab === item.key; return <button key={item.key} type="button" onClick={() => setTab(item.key)} className="kvm-action-button flex items-center gap-2 rounded-lg px-3 py-2 text-sm" style={{ background: active ? "rgba(59,130,246,0.15)" : "transparent", border: active ? "1px solid rgba(59,130,246,0.32)" : "1px solid transparent", color: active ? "var(--kvm-accent-text)" : "var(--kvm-text-muted)" }}><Icon size={15} />{item.label}</button>; })}</div><PageSizePicker value={pageSize} onChange={setPageSize} /></div>
      <div className="rounded-xl p-3" style={{ background: "var(--kvm-card)", border: "1px solid var(--kvm-border)" }}><div className="flex flex-wrap items-center justify-between gap-3"><div className="relative min-w-[260px] flex-1"><SearchIcon size={14} className="absolute left-3 top-1/2 -translate-y-1/2" style={{ color: "var(--kvm-text-muted)" }} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={searchPlaceholders[tab]} className="w-full rounded-lg py-2 pl-9 pr-3 text-sm outline-none" style={{ background: "var(--kvm-control-bg)", border: "1px solid var(--kvm-border)", color: "var(--kvm-text)" }} /></div><button type="button" onClick={() => setAdvancedOpen((open) => !open)} className="kvm-action-button flex h-10 shrink-0 items-center gap-2 rounded-lg border px-3 text-sm" style={{ background: hasAdvancedFilter ? "rgba(59,130,246,0.15)" : "rgba(255,255,255,0.03)", borderColor: hasAdvancedFilter || advancedOpen ? "rgba(59,130,246,0.38)" : "var(--kvm-border)", color: hasAdvancedFilter || advancedOpen ? "var(--kvm-accent-text)" : "var(--kvm-text-muted)" }} aria-expanded={advancedOpen}><FilterIcon size={14} />高级筛选{hasAdvancedFilter && <span className="rounded-md px-1.5 py-0.5 text-[11px]" style={{ background: "rgba(59,130,246,0.16)", color: "var(--kvm-accent-text)" }}>已启用</span>}</button>{tab === "tasks" && <Segmented value={status} onChange={setStatus} items={taskStatuses} />}{tab === "alerts" && <Segmented value={status} onChange={setStatus} items={alertStatuses} />}</div>{advancedOpen && <AdvancedJsonFilter filter={currentJsonFilter} labels={jsonFilterLabels[tab]} onChange={updateJsonFilter} onClear={clearJsonFilter} />}</div>
      {error && <div className="rounded-xl p-4 text-sm" style={{ background: "rgba(245,158,11,0.1)", border: "1px solid rgba(245,158,11,0.25)", color: "#f59e0b" }}>{error}</div>}
      {loading && <div className="text-sm" style={{ color: "var(--kvm-text-muted)" }}>正在加载记录...</div>}
      {!loading && tab === "tasks" && <TaskTable items={visibleTasks} onDetail={(item) => setDetail({ type: "tasks", item })} />}
      {!loading && tab === "audit" && <AuditTable items={visibleLogs} onDetail={(item) => setDetail({ type: "audit", item })} />}
      {!loading && tab === "alerts" && <AlertTable items={visibleAlerts} busyId={busyId} canManage={canManageAlerts} onResolve={handleResolveAlert} onDetail={(item) => setDetail({ type: "alerts", item })} />}
      {!loading && <Pagination start={start} end={end} total={effectiveTotal} pageSize={pageSize} currentPage={currentPage} pageCount={pageCount} setPage={setPage} />}
      {detail && <DetailDialog detail={detail} onClose={() => setDetail(null)} />}
      {tab === "tasks" && <ExportDialog open={exportOpen} title="导出任务" defaultName={`任务-${localTimestamp()}`} rows={exportTasks} columns={taskExportColumns} loading={exportLoading} onClose={() => setExportOpen(false)} />}
      {tab === "audit" && <ExportDialog open={exportOpen} title="导出审计" defaultName={`审计-${localTimestamp()}`} rows={exportLogs} columns={auditExportColumns} loading={exportLoading} onClose={() => setExportOpen(false)} />}
      {tab === "alerts" && <ExportDialog open={exportOpen} title="导出告警" defaultName={`告警-${localTimestamp()}`} rows={exportAlerts} columns={alertExportColumns} loading={exportLoading} onClose={() => setExportOpen(false)} />}
    </div>
  );
}

function TaskTable({ items, onDetail }: { items: Task[]; onDetail: (item: Task) => void }) { return <RecordShell fixed><colgroup><col className="w-[15%]" /><col className="w-[10%]" /><col className="w-[50%]" /><col className="w-[10%]" /><col className="w-[10%]" /><col className="w-[5%]" /></colgroup><thead><tr>{["类型", "状态", "目标", "进度", "时间", "查看"].map((head) => <Head key={head}>{head}</Head>)}</tr></thead><tbody>{items.length === 0 && <Empty colSpan={6} text="暂无任务记录" />}{items.map((item) => <tr key={item.id} style={{ borderBottom: "1px solid rgba(56,78,120,0.16)" }}><Cell main>{labelTaskType(item.type)}</Cell><Cell><StatusPill value={labelStatus(item.status)} tone={statusTone(item.status)} /></Cell><Cell compact><span className="break-words">{item.targetType || "-"}/{item.targetId || "-"}</span></Cell><Cell>{taskProgress(item)}</Cell><Cell>{formatTimeAgo(item.created_at)}</Cell><Cell><TooltipButton label="查看详情" onClick={() => onDetail(item)}><EyeIcon size={14} /></TooltipButton></Cell></tr>)}</tbody></RecordShell>; }
function AuditTable({ items, onDetail }: { items: AuditLog[]; onDetail: (item: AuditLog) => void }) { return <RecordShell><thead><tr>{["动作", "用户", "资源", "IP", "时间", "查看"].map((head) => <Head key={head}>{head}</Head>)}</tr></thead><tbody>{items.length === 0 && <Empty colSpan={6} text="暂无审计日志" />}{items.map((item) => <tr key={item.id} style={{ borderBottom: "1px solid rgba(56,78,120,0.16)" }}><Cell main>{labelAction(item.action)}</Cell><Cell>{item.username || "-"}</Cell><Cell>{item.resourceType}/{item.resourceId || "-"}</Cell><Cell>{item.ipAddress || "-"}</Cell><Cell>{formatTimeAgo(item.created_at)}</Cell><Cell><TooltipButton label="查看详情" onClick={() => onDetail(item)}><EyeIcon size={14} /></TooltipButton></Cell></tr>)}</tbody></RecordShell>; }
function AlertTable({ items, busyId, canManage, onResolve, onDetail }: { items: Alert[]; busyId: string; canManage: boolean; onResolve: (item: Alert) => void; onDetail: (item: Alert) => void }) { return <RecordShell><thead><tr>{["级别", "状态", "标题", "来源", "外部通知", "时间", "操作"].map((head) => <Head key={head}>{head}</Head>)}</tr></thead><tbody>{items.length === 0 && <Empty colSpan={7} text="暂无告警" />}{items.map((item) => <tr key={item.id} style={{ borderBottom: "1px solid rgba(56,78,120,0.16)" }}><Cell main><AlertLevelText level={item.level} /></Cell><Cell><StatusPill value={labelStatus(item.status)} tone={statusTone(item.status)} /></Cell><Cell>{item.title}</Cell><Cell>{labelResource(item.sourceType)}/{item.sourceId}</Cell><Cell>{item.notificationSentAt ? "已触达" : "待发送"}</Cell><Cell>{formatTimeAgo(item.lastSeenAt)}</Cell><Cell><div className="flex justify-center gap-1.5"><TooltipButton label="查看详情" onClick={() => onDetail(item)}><EyeIcon size={14} /></TooltipButton>{canManage && <TooltipButton label="解决告警" disabled={item.status !== "active" || busyId === item.id} success onClick={() => onResolve(item)}><CheckCircle2Icon size={14} /></TooltipButton>}</div></Cell></tr>)}</tbody></RecordShell>; }

function RecordShell({ children, fixed }: { children: React.ReactNode; fixed?: boolean }) { return <div className="overflow-hidden rounded-xl" style={{ background: "var(--kvm-card)", border: "1px solid var(--kvm-border)" }}><div className="overflow-x-auto"><table className={`${fixed ? "table-fixed" : ""} w-full text-sm`}>{children}</table></div></div>; }
function Head({ children }: { children: React.ReactNode }) { return <th className="px-4 py-3 text-center text-xs font-semibold" style={{ color: "var(--kvm-text-muted)", borderBottom: "1px solid var(--kvm-border)" }}>{children}</th>; }
function Cell({ children, main, compact }: { children: React.ReactNode; main?: boolean; compact?: boolean }) { return <td className={`${compact ? "px-2" : "px-4"} py-3 text-center`} style={{ color: main ? "var(--kvm-text)" : "var(--kvm-text-muted)" }}>{children}</td>; }
function Empty({ colSpan, text }: { colSpan: number; text: string }) { return <tr><td colSpan={colSpan} className="px-4 py-10 text-center" style={{ color: "var(--kvm-text-muted)" }}>{text}</td></tr>; }

function AdvancedJsonFilter({ filter, labels, onChange, onClear }: { filter: JsonFilter; labels: { title: string; keyPlaceholder: string; valuePlaceholder: string; sourceName: string }; onChange: (field: keyof JsonFilter, value: string) => void; onClear: () => void }) {
  const active = filter.key.trim() !== "" || filter.value.trim() !== "";
  return <div className="mt-3 rounded-lg border p-3" style={{ background: "rgba(255,255,255,0.025)", borderColor: "var(--kvm-border)" }}><div className="mb-2 flex flex-wrap items-center justify-between gap-2"><div className="flex items-center gap-2 text-xs font-semibold" style={{ color: "var(--kvm-text)" }}><FilterIcon size={13} />{labels.title}</div>{active && <button type="button" onClick={onClear} className="kvm-action-button rounded-md border px-2 py-1 text-xs" style={{ background: "transparent", borderColor: "var(--kvm-border)", color: "var(--kvm-text-muted)" }}>清空</button>}</div><div className="grid grid-cols-1 gap-2 md:grid-cols-[minmax(160px,220px)_1fr]"><input value={filter.key} onChange={(event) => onChange("key", event.target.value)} placeholder={labels.keyPlaceholder} className="h-9 rounded-lg px-3 text-sm outline-none" style={{ background: "var(--kvm-control-bg)", border: "1px solid var(--kvm-border)", color: "var(--kvm-text)" }} /><input value={filter.value} onChange={(event) => onChange("value", event.target.value)} placeholder={labels.valuePlaceholder} className="h-9 rounded-lg px-3 text-sm outline-none" style={{ background: "var(--kvm-control-bg)", border: "1px solid var(--kvm-border)", color: "var(--kvm-text)" }} /></div><div className="mt-2 text-[11px]" style={{ color: "var(--kvm-text-muted)" }}>{filter.key.trim() ? `仅匹配 ${labels.sourceName} 顶层字段 ${filter.key.trim()}` : `字段为空时在整段 ${labels.sourceName} 中搜索值`}</div></div>;
}

function TooltipButton({ label, disabled, success, onClick, children }: { label: string; disabled?: boolean; success?: boolean; onClick: () => void; children: React.ReactNode }) {
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const [tooltip, setTooltip] = useState<{ open: boolean; top: number; left: number; placement: "top" | "bottom" }>({ open: false, top: 0, left: 0, placement: "top" });
  function showTooltip() {
    const trigger = triggerRef.current;
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const placement = rect.top < 50 ? "bottom" : "top";
    setTooltip({ open: true, top: placement === "top" ? rect.top - 8 : rect.bottom + 8, left: Math.min(window.innerWidth - 72, Math.max(72, rect.left + rect.width / 2)), placement });
  }
  return <><button ref={triggerRef} type="button" aria-label={label} disabled={disabled} onMouseEnter={showTooltip} onMouseLeave={() => setTooltip((current) => ({ ...current, open: false }))} onFocus={showTooltip} onBlur={() => setTooltip((current) => ({ ...current, open: false }))} onClick={onClick} className="kvm-action-button inline-flex h-8 w-8 items-center justify-center rounded-lg border disabled:opacity-40" style={{ borderColor: success ? "rgba(16,185,129,0.35)" : "var(--kvm-border)", color: success ? "#34d399" : "var(--kvm-text-muted)", background: "rgba(255,255,255,0.03)" }}>{children}</button>{tooltip.open && <div className="pointer-events-none fixed z-[80] -translate-x-1/2 text-left shadow-2xl" style={{ left: tooltip.left, top: tooltip.top, transform: tooltip.placement === "top" ? "translate(-50%, -100%)" : "translate(-50%, 0)" }}><div className="whitespace-nowrap rounded-lg border px-2.5 py-1.5 text-xs font-semibold" style={{ background: "var(--kvm-popover-bg)", borderColor: "var(--kvm-popover-border)", color: "var(--kvm-text)" }}>{label}</div></div>}</>;
}

function Segmented({ value, onChange, items }: { value: string; onChange: (value: string) => void; items: Array<{ value: string; label: string }> }) { return <div className="flex flex-wrap gap-2">{items.map((item) => <button key={item.value} type="button" onClick={() => onChange(item.value)} className="kvm-action-button rounded-lg px-3 py-2 text-xs" style={{ background: value === item.value ? "rgba(59,130,246,0.15)" : "transparent", border: value === item.value ? "1px solid rgba(59,130,246,0.32)" : "1px solid var(--kvm-border)", color: value === item.value ? "var(--kvm-accent-text)" : "var(--kvm-text-muted)" }}>{item.label}</button>)}</div>; }
function PageSizePicker({ value, onChange }: { value: PageSize; onChange: (value: PageSize) => void }) { const [open, setOpen] = useState(false); const rootRef = useRef<HTMLDivElement | null>(null); const current = pageSizes.find((item) => item.value === value) ?? pageSizes[1]; useEffect(() => { const close = (event: MouseEvent) => { if (!rootRef.current?.contains(event.target as Node)) setOpen(false); }; window.addEventListener("mousedown", close); return () => window.removeEventListener("mousedown", close); }, []); return <div ref={rootRef} className="relative flex items-center gap-2"><span className="text-sm" style={{ color: "var(--kvm-text-muted)" }}>显示数量</span><button type="button" onClick={() => setOpen((next) => !next)} className="kvm-action-button flex h-10 w-28 cursor-pointer items-center justify-between rounded-lg border px-3 text-left" style={{ background: "rgba(255,255,255,0.045)", borderColor: open ? "rgba(96,165,250,0.48)" : "var(--kvm-border)", color: "var(--kvm-text)" }}><span className="truncate text-sm font-semibold">{current.label}</span><ChevronDownIcon size={15} className={open ? "rotate-180 transition-transform" : "transition-transform"} style={{ color: "var(--kvm-text-muted)" }} /></button>{open && <div className="absolute right-0 top-[calc(100%+8px)] z-40 w-28 overflow-hidden rounded-lg border p-1 shadow-2xl" style={{ background: "var(--kvm-menu-bg)", borderColor: "var(--kvm-popover-border)", boxShadow: "var(--kvm-menu-shadow)" }}>{pageSizes.map((item) => <button key={String(item.value)} type="button" onClick={() => { onChange(item.value); setOpen(false); }} className="flex h-10 w-full cursor-pointer items-center justify-between rounded-md px-3 text-left text-sm font-semibold" style={{ background: item.value === value ? "rgba(96,165,250,0.16)" : undefined, color: item.value === value ? "#bfdbfe" : "var(--kvm-text)" }}><span>{item.label}</span>{item.value === value && <CheckIcon size={15} />}</button>)}</div>}</div>; }
function Pagination({ start, end, total, pageSize, currentPage, pageCount, setPage }: { start: number; end: number; total: number; pageSize: PageSize; currentPage: number; pageCount: number; setPage: React.Dispatch<React.SetStateAction<number>> }) { return <div className="flex flex-wrap items-center justify-between gap-3 text-sm" style={{ color: "var(--kvm-text-muted)" }}><span>显示 {start}-{end} 条，共 {total} 条</span>{pageSize !== "all" && <div className="flex items-center gap-2"><PageButton label="首页" disabled={currentPage <= 1} onClick={() => setPage(1)}><ChevronsLeftIcon size={16} /></PageButton><PageButton label="上一页" disabled={currentPage <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))}><ChevronLeftIcon size={16} /></PageButton><span className="min-w-[76px] text-center">{currentPage} / {pageCount}</span><PageButton label="下一页" disabled={currentPage >= pageCount} onClick={() => setPage((value) => Math.min(pageCount, value + 1))}><ChevronRightIcon size={16} /></PageButton><PageButton label="尾页" disabled={currentPage >= pageCount} onClick={() => setPage(pageCount)}><ChevronsRightIcon size={16} /></PageButton></div>}</div>; }
function PageButton({ label, disabled, onClick, children }: { label: string; disabled: boolean; onClick: () => void; children: React.ReactNode }) { return <button type="button" onClick={onClick} disabled={disabled} className="kvm-action-button flex h-9 w-9 items-center justify-center rounded-lg border disabled:opacity-40" style={{ background: "var(--kvm-card)", borderColor: "var(--kvm-border)", color: "var(--kvm-text-muted)" }} aria-label={label}>{children}</button>; }

function DetailDialog({ detail, onClose }: { detail: DetailItem; onClose: () => void }) {
  if (!detail) return null;
  const title = detail.type === "tasks" ? "任务详情" : detail.type === "audit" ? "审计详情" : "告警详情";
  const rows = detailRows(detail);
  const extra = extraPayload(detail);
  return <DialogPortal><div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-4"><div className="kvm-dialog-panel kvm-operations-dialog w-full max-w-3xl rounded-xl p-5 shadow-2xl"><div className="mb-4 flex items-center justify-between"><h3 className="text-base font-semibold" style={{ color: "var(--kvm-text)" }}>{title}</h3><button type="button" onClick={onClose} className="kvm-action-button flex h-8 w-8 items-center justify-center rounded-lg border" style={{ borderColor: "var(--kvm-border)", color: "var(--kvm-text-muted)", background: "rgba(255,255,255,0.03)" }} aria-label="关闭"><XIcon size={15} /></button></div><div className="kvm-detail-tile-grid grid grid-cols-1 gap-3 md:grid-cols-2">{rows.map((row) => <ReadOnlyField key={row.label} label={row.label} value={row.value} />)}</div>{extra && <div className="mt-4"><div className="mb-2 text-xs font-semibold" style={{ color: "var(--kvm-text-muted)" }}>{extra.title}</div><pre className="kvm-payload-pre kvm-hidden-scrollbar max-h-[32vh] overflow-y-auto overflow-x-hidden whitespace-pre-wrap break-all rounded-lg p-4 text-xs" style={{ color: "var(--kvm-text-muted)" }}>{extra.value}</pre></div>}</div></div></DialogPortal>;
}

function ReadOnlyField({ label, value }: { label: string; value: React.ReactNode }) { return <div className="kvm-dialog-card kvm-operations-detail-card rounded-lg p-3"><div className="mb-1 text-[11px]" style={{ color: "var(--kvm-text-muted)" }}>{label}</div><div className="break-words text-sm font-medium" style={{ color: "var(--kvm-text)" }}>{value || "-"}</div></div>; }
function StatusPill({ value, tone }: { value: string; tone: "green" | "red" | "yellow" | "blue" | "gray" }) { const colors = { green: ["var(--kvm-status-green-text)", "rgba(16,185,129,0.12)", "rgba(16,185,129,0.24)"], red: ["var(--kvm-status-red-text)", "rgba(239,68,68,0.12)", "rgba(239,68,68,0.24)"], yellow: ["var(--kvm-status-yellow-text)", "rgba(245,158,11,0.12)", "rgba(245,158,11,0.24)"], blue: ["var(--kvm-status-blue-text)", "rgba(59,130,246,0.12)", "rgba(59,130,246,0.24)"], gray: ["var(--kvm-status-gray-text)", "rgba(148,163,184,0.1)", "rgba(148,163,184,0.18)"] }[tone]; return <span className="inline-flex rounded-md px-2 py-1 text-[11px] font-semibold" style={{ color: colors[0], background: colors[1], border: `1px solid ${colors[2]}` }}>{value}</span>; }
function AlertLevelText({ level }: { level: string }) { return <span className="inline-flex items-center justify-center gap-1.5 font-semibold" style={{ color: alertLevelColor(level) }}><AlertTriangleIcon size={14} />{labelAlertLevel(level)}</span>; }

function detailRows(detail: NonNullable<DetailItem>) {
  if (detail.type === "tasks") { const item = detail.item as Task; return [{ label: "任务 ID", value: item.id }, { label: "任务类型", value: labelTaskType(item.type) }, { label: "状态", value: labelStatus(item.status) }, { label: "目标", value: `${item.targetType || "-"}/${item.targetId || "-"}` }, { label: "进度", value: taskProgress(item) }, { label: "错误信息", value: item.errorMessage || "-" }, { label: "创建时间", value: fullTime(item.created_at) }, { label: "完成时间", value: item.finished_at ? fullTime(item.finished_at) : "-" }]; }
  if (detail.type === "audit") { const item = detail.item as AuditLog; return [{ label: "审计 ID", value: item.id }, { label: "动作", value: labelAction(item.action) }, { label: "用户", value: item.username || "-" }, { label: "用户 ID", value: item.userId || "-" }, { label: "资源", value: `${item.resourceType}/${item.resourceId || "-"}` }, { label: "IP 地址", value: item.ipAddress || "-" }, { label: "发生时间", value: fullTime(item.created_at) }]; }
  const item = detail.item as Alert; return [{ label: "告警 ID", value: item.id }, { label: "级别", value: <AlertLevelText level={item.level} /> }, { label: "状态", value: labelStatus(item.status) }, { label: "标题", value: item.title }, { label: "消息", value: item.message }, { label: "来源", value: `${labelResource(item.sourceType)}/${item.sourceId}` }, { label: "外部通知", value: item.notificationSentAt ? fullTime(item.notificationSentAt) : "待发送" }, { label: "首次出现", value: fullTime(item.firstSeenAt) }, { label: "最近出现", value: fullTime(item.lastSeenAt) }, { label: "解决时间", value: item.resolvedAt ? fullTime(item.resolvedAt) : "-" }];
}
function extraPayload(detail: NonNullable<DetailItem>) { const item = detail.item as Task | AuditLog | Alert; const payload = detail.type === "tasks" ? (item as Task).payload : detail.type === "audit" ? (item as AuditLog).metadata : (item as Alert).metadata; const parsed = parsePayload(payload); if (!parsed) return null; return { title: detail.type === "tasks" ? "任务载荷" : detail.type === "audit" ? "审计元数据" : "告警元数据", value: JSON.stringify(parsed, null, 2) }; }
function parsePayload(value: unknown) { if (!value) return null; if (typeof value === "string") { try { return JSON.parse(value); } catch { return value; } } return value; }
function exportPayload(value: unknown) { const parsed = parsePayload(value); if (!parsed) return "-"; return typeof parsed === "object" ? JSON.stringify(parsed) : String(parsed); }
function fullTime(value?: string) { if (!value) return "-"; const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString(); }
function filterItems<T>(items: T[], query: string, selector: (item: T) => string) { const keyword = query.trim().toLowerCase(); if (!keyword) return items; return items.filter((item) => selector(item).toLowerCase().includes(keyword)); }
function taskProgress(task: Task) { const payload = parsePayload(task.payload) as { totalAgents?: number; syncedAgents?: number; failedAgents?: number; message?: string } | undefined; if (!payload?.totalAgents) return payload?.message || "-"; return `${payload.syncedAgents ?? 0}/${payload.totalAgents}，异常 ${payload.failedAgents ?? 0}`; }
function labelStatus(status: string) { return { queued: "排队中", running: "运行中", completed: "已完成", failed: "失败", active: "活跃", resolved: "已解决" }[status] ?? status; }
function statusTone(status: string): "green" | "red" | "yellow" | "blue" | "gray" { if (["completed", "resolved"].includes(status)) return "green"; if (status === "failed") return "red"; if (["queued", "active"].includes(status)) return "yellow"; if (status === "running") return "blue"; return "gray"; }
function labelAlertLevel(level: string) { return { info: "信息", warning: "警告", critical: "严重" }[level] ?? level; }
function alertLevelColor(level: string) { return { info: "#93c5fd", warning: "#fcd34d", critical: "#fca5a5" }[level] ?? "var(--kvm-text)"; }
function labelResource(type: string) { return { agent: "Agent", host: "宿主机", virtual_machine: "虚拟机", system: "系统", snapshot: "快照" }[type] ?? type; }
function labelNotificationStatus(item: Alert) { return item.notificationSentAt ? "已触达" : "待发送"; }
function labelTaskType(type: string) { return type; }
function labelAction(action: string) { return action; }
function isPermissionMessage(message: string) { return message.includes("当前用户无权执行此操作"); }

const taskExportColumns: ExportColumn<Task>[] = [
  { id: "id", header: "任务 ID", value: item => item.id },
  { id: "type", header: "类型", value: item => labelTaskType(item.type) },
  { id: "status", header: "状态", value: item => labelStatus(item.status) },
  { id: "target", header: "目标", value: item => `${item.targetType || "-"}/${item.targetId || "-"}` },
  { id: "progress", header: "进度", value: item => taskProgress(item) },
  { id: "errorMessage", header: "错误信息", value: item => item.errorMessage || "-" },
  { id: "createdAt", header: "创建时间", value: item => fullTime(item.created_at) },
  { id: "finishedAt", header: "完成时间", value: item => item.finished_at ? fullTime(item.finished_at) : "-" },
];

const auditExportColumns: ExportColumn<AuditLog>[] = [
  { id: "id", header: "审计 ID", value: item => item.id },
  { id: "action", header: "动作", value: item => labelAction(item.action) },
  { id: "username", header: "用户", value: item => item.username || "-" },
  { id: "userId", header: "用户 ID", value: item => item.userId || "-" },
  { id: "resource", header: "资源", value: item => `${item.resourceType}/${item.resourceId || "-"}` },
  { id: "ip", header: "IP", value: item => item.ipAddress || "-" },
  { id: "createdAt", header: "时间", value: item => fullTime(item.created_at) },
  { id: "metadata", header: "元数据", value: item => exportPayload(item.metadata) },
];

const alertExportColumns: ExportColumn<Alert>[] = [
  { id: "id", header: "告警 ID", value: item => item.id },
  { id: "level", header: "级别", value: item => labelAlertLevel(item.level) },
  { id: "status", header: "状态", value: item => labelStatus(item.status) },
  { id: "title", header: "标题", value: item => item.title },
  { id: "message", header: "消息", value: item => item.message },
  { id: "source", header: "来源", value: item => `${labelResource(item.sourceType)}/${item.sourceId}` },
  { id: "notification", header: "外部通知", value: item => labelNotificationStatus(item) },
  { id: "firstSeenAt", header: "首次出现", value: item => fullTime(item.firstSeenAt) },
  { id: "lastSeenAt", header: "最近出现", value: item => fullTime(item.lastSeenAt) },
  { id: "metadata", header: "元数据", value: item => exportPayload(item.metadata) },
];
