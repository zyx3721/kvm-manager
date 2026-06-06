import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { fetchHosts, fetchVMConsoleInfo, fetchVMs, refreshVM, runVMAction, unmarkVMTemplate, type Host, type VirtualMachine } from '../../lib/api';
import { VMEditDialog } from './components/VMEditDialog';
import { VMCloneDialog } from './components/VMCloneDialog';
import { VMMonitorDialog } from './components/VMMonitorDialog';
import { onKvmRefresh } from '../../lib/refresh';
import { ActionDialog } from './components/ActionDialog';
import { VMCreateDialog } from './components/VMCreateDialog';
import { VMMigrateDialog } from './components/VMMigrateDialog';
import { VMBulkActionBar } from './components/VMBulkActionBar';
import { VMBulkConfirmDialog } from './components/VMBulkConfirmDialog';
import { VMToolbar, type VMColumnOption, type VMFilterOption } from './components/VMToolbar';
import { VMTable, type VMTableColumnKey } from './components/VMTable';
import { VMTemplateMarkDialog } from './components/VMTemplateMarkDialog';
import { VMViewSwitch, type VMViewMode } from './components/VMViewSwitch';
import { ExportDialog } from '../../components/kvm/ExportDialog';
import { ConsolePasswordDialog } from './components/ConsolePasswordDialog';
import { actionMeta, type PendingAction, type VMBulkAction, type VMAction } from './types';
import { can, vmActionPermission } from '../../lib/permissions';
import { formatBytes, formatMemoryBytes, formatUptime } from '../../lib/format';
import { localTimestamp, type ExportColumn } from '../../lib/exportData';
import { isVMPaused, isVMRunning, isVMStopped, vmStatusLabel } from './utils/vmStatus';

const filters: VMFilterOption[] = [
  { key: 'all', label: '全部' },
  { key: 'running', label: '运行中' },
  { key: 'stopped', label: '已停止' },
  { key: 'paused', label: '已暂停' },
  { key: 'error', label: '异常' },
];

const vmColumnOptions: VMColumnOption[] = [
  { key: 'identity', label: '名称 / IP' },
  { key: 'description', label: '描述' },
  { key: 'status', label: '状态' },
  { key: 'system', label: '系统' },
  { key: 'host', label: '宿主机' },
  { key: 'cpu', label: 'CPU' },
  { key: 'memory', label: '内存' },
  { key: 'disk', label: '磁盘' },
  { key: 'uptime', label: '运行时长' },
];

const templateColumnOptions: VMColumnOption[] = [
  { key: 'template', label: '模板名称 / 描述' },
  { key: 'source', label: '源虚拟机 / IP' },
  { key: 'status', label: '状态' },
  { key: 'system', label: '系统' },
  { key: 'host', label: '宿主机' },
  { key: 'cpu', label: 'CPU' },
  { key: 'memory', label: '内存' },
  { key: 'disk', label: '磁盘' },
];

export default function VMs() {
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState('all');
  const [viewMode, setViewMode] = useState<VMViewMode>('vms');
  const [hostFilter, setHostFilter] = useState('all');
  const [vms, setVMs] = useState<VirtualMachine[]>([]);
  const [allVMs, setAllVMs] = useState<VirtualMachine[]>([]);
  const [rawAllVMs, setRawAllVMs] = useState<VirtualMachine[]>([]);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshingId, setRefreshingId] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [actionId, setActionId] = useState<string | null>(null);
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null);
  const [consolePasswordVM, setConsolePasswordVM] = useState<VirtualMachine | null>(null);
  const [consolePassword, setConsolePassword] = useState('');
  const [consolePasswordBusy, setConsolePasswordBusy] = useState(false);
  const [confirmName, setConfirmName] = useState('');
  const [monitorVM, setMonitorVM] = useState<VirtualMachine | null>(null);
  const [editingVM, setEditingVM] = useState<VirtualMachine | null>(null);
  const [cloningVM, setCloningVM] = useState<VirtualMachine | null>(null);
  const [markingTemplateVM, setMarkingTemplateVM] = useState<VirtualMachine | null>(null);
  const [templateCreateVM, setTemplateCreateVM] = useState<VirtualMachine | null>(null);
  const [migratingVM, setMigratingVM] = useState<VirtualMachine | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [bulkAction, setBulkAction] = useState<VMBulkAction>('start');
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkConfirmOpen, setBulkConfirmOpen] = useState(false);
  const [bulkConfirmText, setBulkConfirmText] = useState('');
  const [exportOpen, setExportOpen] = useState(false);
  const [exportRows, setExportRows] = useState<VirtualMachine[]>([]);
  const [vmVisibleColumns, setVMVisibleColumns] = useState<VMTableColumnKey[]>(() => vmColumnOptions.map(option => option.key));
  const [templateVisibleColumns, setTemplateVisibleColumns] = useState<VMTableColumnKey[]>(() => templateColumnOptions.map(option => option.key));
  const [shutdownMode, setShutdownMode] = useState<'stop' | 'force-stop' | 'shutdown' | 'force-shutdown'>('shutdown');
  const [rebootMode, setRebootMode] = useState<'reboot' | 'force-reboot'>('reboot');
  const [deleteMode, setDeleteMode] = useState<'delete' | 'force-delete'>('delete');
  const loadSeqRef = useRef(0);
  const permissions = useMemo(() => ({
    create: can('vms.create'),
    update: can('vms.update'),
    power: can('vms.power'),
    delete: can('vms.delete'),
    force: can('vms.force'),
    console: can('vms.console'),
    clone: can('vms.clone'),
    migrate: can('vms.migrate'),
  }), []);

  const loadVMs = useCallback(async () => {
    const seq = ++loadSeqRef.current;
    setLoading(true);
    setError('');
    try {
      const query = search.trim();
      const [displayResponse, countResponse] = await Promise.all([
        fetchVMs({ status: filter, q: query, hostId: hostFilter }),
        fetchVMs({ q: query, hostId: hostFilter }),
      ]);
      if (seq !== loadSeqRef.current) return;
      const displayItems = filterByViewMode(displayResponse.items, viewMode);
      const countItems = filterByViewMode(countResponse.items, viewMode);
      setVMs(displayItems);
      setAllVMs(countItems);
      setRawAllVMs(countResponse.items);
    } catch (err) {
      if (seq !== loadSeqRef.current) return;
      const message = err instanceof Error ? err.message : '读取虚拟机列表失败';
      toast.error(message);
      setError(isPermissionMessage(message) ? '' : message);
    } finally {
      if (seq === loadSeqRef.current) setLoading(false);
    }
  }, [filter, hostFilter, search, viewMode]);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadVMs(), 250);
    const unsubscribe = onKvmRefresh(() => {
      void loadVMs();
    });
    return () => {
      window.clearTimeout(timer);
      unsubscribe();
    };
  }, [loadVMs]);

  useEffect(() => {
    fetchHosts()
      .then(response => setHosts(response.items))
      .catch(err => {
        const message = err instanceof Error ? err.message : '读取宿主机列表失败';
        if (!isPermissionMessage(message)) toast.error(message);
      });
  }, []);

  useEffect(() => {
    setSelectedIds(current => current.filter(id => vms.some(vm => vm.id === id)));
  }, [vms]);

  const counts = useMemo(
    () => ({
      all: allVMs.length,
      running: allVMs.filter(vm => isVMRunning(vm.status)).length,
      stopped: allVMs.filter(vm => isVMStopped(vm.status)).length,
      paused: allVMs.filter(vm => isVMPaused(vm.status)).length,
      error: allVMs.filter(vm => vm.status === 'error').length,
    }),
    [allVMs]
  );

  const selectedVMs = useMemo(
    () => selectedIds.map(id => vms.find(vm => vm.id === id)).filter((vm): vm is VirtualMachine => Boolean(vm)),
    [selectedIds, vms]
  );
  const hostOptions = useMemo(
    () => [
      { value: 'all', label: '全部宿主机' },
      ...hosts.map(host => ({
        value: host.id,
        label: host.name || host.address || host.hostname || host.id,
        tooltip: [host.address, host.hostname, host.status].filter(Boolean).join(' · '),
      })),
    ],
    [hosts]
  );
  const activeColumnOptions = viewMode === 'templates' ? templateColumnOptions : vmColumnOptions;
  const activeVisibleColumns = viewMode === 'templates' ? templateVisibleColumns : vmVisibleColumns;

  function changeVisibleColumns(columns: VMTableColumnKey[]) {
    if (viewMode === 'templates') {
      setTemplateVisibleColumns(columns);
      return;
    }
    setVMVisibleColumns(columns);
  }

  async function openAction(vm: VirtualMachine, action: VMAction | 'console') {
    if (action === 'console' && !permissions.console) {
      toast.error('当前用户无权执行此操作');
      return;
    }
    if (action !== 'console' && !can(vmActionPermission(action))) {
      toast.error('当前用户无权执行此操作');
      return;
    }
    if (action === 'console') {
      try {
        const info = await fetchVMConsoleInfo(vm.id);
        if (info.type && info.type !== 'vnc') {
          toast.warning('当前仅支持 VNC 控制台');
          return;
        }
        if (info.passwordEnabled && isVMRunning(vm.status)) {
          setConsolePasswordVM(vm);
          setConsolePassword('');
          return;
        }
      } catch (err) {
        toast.error(err instanceof Error ? err.message : '读取控制台配置失败');
        return;
      }
      setConsolePassword('');
    }
    setPendingAction({ vm, action });
    setConfirmName('');
    setShutdownMode(action === 'shutdown' ? 'shutdown' : 'stop');
    setRebootMode('reboot');
    setDeleteMode('delete');
  }

  function confirmConsolePassword(password: string) {
    if (!consolePasswordVM) return;
    setConsolePasswordBusy(true);
    setConsolePassword(password);
    setPendingAction({ vm: consolePasswordVM, action: 'console' });
    setConsolePasswordVM(null);
    setConsolePasswordBusy(false);
  }

  async function confirmAction() {
    if (!pendingAction || pendingAction.action === 'console') return;
    const vm = pendingAction.vm;
    const action =
      pendingAction.action === 'stop' || pendingAction.action === 'shutdown'
        ? shutdownMode
        : pendingAction.action === 'reboot'
          ? rebootMode
          : pendingAction.action === 'delete'
            ? deleteMode
          : pendingAction.action;
    if ((action === 'delete' || action === 'force-delete') && confirmName.trim() !== vm.name) {
      toast.error('请输入虚拟机名称以确认删除');
      return;
    }
    if (action === 'delete' && !isVMStopped(vm.status)) {
      toast.error('请先关闭虚拟机再进行删除');
      return;
    }
    setActionId(vm.id);
    setPendingAction(null);
    const toastId = toast.loading(`${vm.name} ${actionMeta[action].label}指令已下发`);
    try {
      await runVMAction(vm.id, action);
      await loadVMs();
      toast.success(`${vm.name} ${actionMeta[action].label}执行完成`, { id: toastId });
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '虚拟机操作失败', { id: toastId });
    } finally {
      setActionId(null);
    }
  }

  async function refreshSingleVM(vm: VirtualMachine) {
    setRefreshingId(vm.id);
    try {
      const response = await refreshVM(vm.id);
      setVMs(items => items.map(item => (item.id === vm.id ? response.vm : item)));
      setAllVMs(items => items.map(item => (item.id === vm.id ? response.vm : item)));
      toast.success(`${vm.name} 信息已刷新`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '刷新虚拟机信息失败');
    } finally {
      setRefreshingId(null);
    }
  }

  async function unmarkTemplate(vm: VirtualMachine) {
    setActionId(vm.id);
    const toastId = toast.loading(`${vm.name} 正在取消模板标记`);
    try {
      await unmarkVMTemplate(vm.id);
      toast.success(`${vm.name} 已取消模板标记`, { id: toastId });
      await loadVMs();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '取消模板标记失败', { id: toastId });
    } finally {
      setActionId(null);
    }
  }

  function toggleSelected(vm: VirtualMachine) {
    setSelectedIds(current =>
      current.includes(vm.id) ? current.filter(id => id !== vm.id) : [...current, vm.id]
    );
  }

  function toggleAllVisible() {
    setSelectedIds(current => {
      if (vms.length > 0 && vms.every(vm => current.includes(vm.id))) {
        return current.filter(id => !vms.some(vm => vm.id === id));
      }
      return Array.from(new Set([...current, ...vms.map(vm => vm.id)]));
    });
  }

  function requestBulkAction() {
    if (selectedVMs.length === 0) return toast.warning('请选择虚拟机');
    if (!can(vmActionPermission(bulkAction))) {
      toast.error('当前用户无权执行此操作');
      return;
    }
    if (bulkAction === 'delete' || bulkAction === 'force-delete') {
      setBulkConfirmText('');
      setBulkConfirmOpen(true);
      return;
    }
    void runBulkAction();
  }

  async function runBulkAction() {
    const targets = selectedVMs;
    if (targets.length === 0) return;
    setBulkBusy(true);
    setBulkConfirmOpen(false);
    setBulkConfirmText('');
    const total = targets.length;
    const toastId = toast.loading(`[0/${total}] 正在批量执行${actionMeta[bulkAction].label}`);
    let success = 0;
    if (total === 1) {
      try {
        const vm = targets[0];
        const action = resolveBulkVMAction(vm, bulkAction);
        await runVMAction(vm.id, action);
        success += 1;
        toast.success(`[${success}/${total}] ${vm.name}: ${actionMeta[action].label}执行完成`, { id: toastId });
        setSelectedIds([]);
        await loadVMs();
      } catch (err) {
        const failedVM = targets[success];
        const failedName = failedVM?.name ?? '未知虚拟机';
        const message = err instanceof Error ? err.message : '批量操作失败';
        toast.error(`[${Math.min(success + 1, total)}/${total}] ${failedName}: ${message}`, { id: toastId });
        await loadVMs();
      } finally {
        setBulkBusy(false);
      }
      return;
    }

    try {
      for (const [index, vm] of targets.entries()) {
        const action = resolveBulkVMAction(vm, bulkAction);
        const progress = index + 1;
        try {
          await runVMAction(vm.id, action);
          success += 1;
          const message = `[${progress}/${total}] ${vm.name}: ${actionMeta[action].label}执行完成`;
          if (progress === total) {
            toast.success(message, { id: toastId });
          } else {
            toast.loading(message, { id: toastId });
          }
        } catch (err) {
          const message = err instanceof Error ? err.message : '批量操作失败';
          const toastMessage = `[${progress}/${total}] ${vm.name}: ${message}`;
          if (progress === total) {
            toast.error(toastMessage, { id: toastId });
          } else {
            toast.loading(toastMessage, { id: toastId });
          }
        }
      }
      setSelectedIds([]);
      await loadVMs();
    } finally {
      setBulkBusy(false);
    }
  }

  async function openExportDialog() {
    try {
      const response = await fetchVMs({ status: filter, q: search.trim(), hostId: hostFilter });
      setExportRows(response.items);
      setExportOpen(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '读取导出数据失败');
    }
  }

  function switchViewMode(next: VMViewMode) {
    loadSeqRef.current += 1;
    setViewMode(next);
    setSelectedIds([]);
    setVMs(filterByStatus(filterByViewMode(rawAllVMs, next), filter));
    setAllVMs(filterByViewMode(rawAllVMs, next));
  }

  return (
    <div data-cmp="VMs" className="w-full space-y-5 p-6">
      <VMToolbar
        search={search}
        filter={filter}
        hostFilter={hostFilter}
        hostOptions={hostOptions}
        filters={filters}
        counts={counts}
        onSearchChange={setSearch}
        onFilterChange={setFilter}
        onHostFilterChange={setHostFilter}
        onCreate={() => setCreateOpen(true)}
        onExport={() => void openExportDialog()}
        canCreate={permissions.create}
        columnOptions={activeColumnOptions}
        visibleColumns={activeVisibleColumns}
        onVisibleColumnsChange={changeVisibleColumns}
        actionsSlot={
          <VMViewSwitch
            value={viewMode}
            vmCount={rawAllVMs.filter(vm => !vm.isTemplate).length}
            templateCount={rawAllVMs.filter(vm => vm.isTemplate).length}
            onChange={switchViewMode}
          />
        }
      >
        {selectedIds.length > 0 && viewMode === 'vms' && (
          <VMBulkActionBar
            selectedCount={selectedIds.length}
            action={bulkAction}
            busy={bulkBusy}
            onActionChange={setBulkAction}
            onRun={requestBulkAction}
            canRun={can(vmActionPermission(bulkAction))}
          />
        )}
      </VMToolbar>

      {error && (
        <div
          className="rounded-xl p-4 text-sm"
          style={{
            background: 'rgba(245,158,11,0.1)',
            border: '1px solid rgba(245,158,11,0.25)',
            color: '#f59e0b',
          }}
        >
          {error}
        </div>
      )}

      <VMTable
        vms={vms}
        hosts={hosts}
        loading={loading}
        selectedIds={selectedIds}
        actionId={actionId}
        refreshingId={refreshingId}
        onToggleSelected={toggleSelected}
        onToggleAllVisible={toggleAllVisible}
        onEdit={setEditingVM}
        onRefresh={refreshSingleVM}
        onMonitor={setMonitorVM}
        onConsole={vm => void openAction(vm, 'console')}
        onClone={setCloningVM}
        onMarkTemplate={setMarkingTemplateVM}
        onUnmarkTemplate={vm => void unmarkTemplate(vm)}
        onCreateFromTemplate={setTemplateCreateVM}
        onMigrate={setMigratingVM}
        onAction={(vm, action) => void openAction(vm, action)}
        permissions={permissions}
        templateView={viewMode === 'templates'}
        visibleColumns={activeVisibleColumns}
      />
      {consolePasswordVM && (
        <ConsolePasswordDialog
          vm={consolePasswordVM}
          busy={consolePasswordBusy}
          onClose={() => setConsolePasswordVM(null)}
          onConfirm={confirmConsolePassword}
        />
      )}
      <ExportDialog
        open={exportOpen}
        title="导出虚拟机"
        defaultName={`虚拟机-${localTimestamp()}`}
        rows={exportRows}
        columns={vmExportColumns}
        onClose={() => setExportOpen(false)}
      />
      {pendingAction && (
        <ActionDialog
          pending={pendingAction}
          confirmName={confirmName}
          shutdownMode={shutdownMode}
          rebootMode={rebootMode}
          deleteMode={deleteMode}
          busy={actionId === pendingAction.vm.id}
          canForce={permissions.force}
          onConfirmNameChange={setConfirmName}
          onShutdownModeChange={setShutdownMode}
          onRebootModeChange={setRebootMode}
          onDeleteModeChange={setDeleteMode}
          onClose={() => setPendingAction(null)}
          onConfirm={() => void confirmAction()}
          consolePassword={consolePassword}
        />
      )}
      {bulkConfirmOpen && (
        <VMBulkConfirmDialog
          action={bulkAction}
          selectedNames={selectedVMs.map(vm => vm.name)}
          confirmText={bulkConfirmText}
          busy={bulkBusy}
          onConfirmTextChange={setBulkConfirmText}
          onClose={() => setBulkConfirmOpen(false)}
          onConfirm={() => void runBulkAction()}
        />
      )}
      {monitorVM && <VMMonitorDialog vm={monitorVM} onClose={() => setMonitorVM(null)} />}
      {markingTemplateVM && (
        <VMTemplateMarkDialog
          vm={markingTemplateVM}
          onClose={() => setMarkingTemplateVM(null)}
          onMarked={() => void loadVMs()}
        />
      )}
      {editingVM && (
        <VMEditDialog
          vm={editingVM}
          onClose={() => setEditingVM(null)}
          onUpdated={() => loadVMs()}
        />
      )}
      {createOpen && (
        <VMCreateDialog
          hosts={hosts}
          onClose={() => {
            setCreateOpen(false);
            void loadVMs();
          }}
        />
      )}
      {migratingVM && (
        <VMMigrateDialog
          vm={migratingVM}
          hosts={hosts}
          onClose={() => {
            setMigratingVM(null);
            void loadVMs();
          }}
        />
      )}
      {cloningVM && (
        <VMCloneDialog
          vm={cloningVM}
          onClose={() => {
            setCloningVM(null);
            void loadVMs();
          }}
        />
      )}
      {templateCreateVM && (
        <VMCloneDialog
          mode="template"
          vm={templateCreateVM}
          onClose={() => {
            setTemplateCreateVM(null);
            void loadVMs();
          }}
        />
      )}
    </div>
  );
}

const vmExportColumns: ExportColumn<VirtualMachine>[] = [
  { id: 'name', header: '名称', value: vm => vm.name },
  { id: 'description', header: '描述', value: vm => vm.templateDescription || vm.description || '-' },
  { id: 'host', header: '宿主机', value: vm => vm.hostName },
  { id: 'status', header: '状态', value: vm => vmStatusLabel(vm.status) },
  { id: 'primaryIp', header: '主 IP', value: vm => vm.primaryIp || '-' },
  { id: 'osType', header: '系统', value: vm => vm.osType || '-' },
  { id: 'cpuCores', header: 'CPU', value: vm => vm.cpuCores },
  { id: 'cpuUsage', header: 'CPU 使用率', value: vm => `${vm.cpuUsage}%` },
  { id: 'memory', header: '内存', value: vm => formatMemoryBytes(vm.memoryBytes) },
  { id: 'memoryUsage', header: '内存使用率', value: vm => `${vm.memoryUsage}%` },
  { id: 'disk', header: '磁盘', value: vm => formatBytes(vm.diskBytes, 'GB') },
  { id: 'diskUsage', header: '磁盘使用率', value: vm => `${vm.diskUsage}%` },
  { id: 'uptime', header: '运行时长', value: vm => formatUptime(vm.uptimeSeconds) },
];

function isPermissionMessage(message: string) {
  return message.includes('当前用户无权执行此操作');
}

function resolveBulkVMAction(vm: VirtualMachine, action: VMBulkAction): VMAction {
  if (action === 'start' && isVMPaused(vm.status)) {
    return 'resume';
  }
  return action;
}

function filterByViewMode(items: VirtualMachine[], viewMode: VMViewMode) {
  return items.filter(vm => (viewMode === 'templates' ? Boolean(vm.isTemplate) : !vm.isTemplate));
}

function filterByStatus(items: VirtualMachine[], status: string) {
  if (status === 'all') return items;
  if (status === 'running') return items.filter(vm => isVMRunning(vm.status));
  if (status === 'stopped') return items.filter(vm => isVMStopped(vm.status));
  if (status === 'paused') return items.filter(vm => isVMPaused(vm.status));
  return items.filter(vm => vm.status === status);
}
