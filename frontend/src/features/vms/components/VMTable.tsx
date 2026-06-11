import {
  BarChart3Icon,
  BadgeCheckIcon,
  CheckIcon,
  CopyPlusIcon,
  Layers3Icon,
  MonitorIcon,
  MoveRightIcon,
  PauseIcon,
  PencilIcon,
  PlayIcon,
  RepeatIcon,
  RotateCwIcon,
  SquareIcon,
  Trash2Icon,
  XCircleIcon,
} from 'lucide-react';

import type { Host, VirtualMachine } from '../../../lib/api';
import { formatMemoryBytes, formatOSType, formatUptime } from '../../../lib/format';
import { StatusBadge } from '../../../components/kvm/StatusBadge';
import { ActionButton } from './ActionButton';
import { DiskMetricCell, VMMetricCell } from './VMMetrics';
import type { VMAction } from '../types';
import { isVMPaused, isVMRunning, isVMStopped } from '../utils/vmStatus';

const tableScrollThreshold = 10;
const tableHeaderHeight = 49;
const tableRowHeight = 92;
const selectColumnWidth = 4;
const defaultVMTableColumns = [
  'identity',
  'description',
  'status',
  'system',
  'host',
  'cpu',
  'memory',
  'disk',
  'uptime',
] as const;
const defaultTemplateTableColumns = [
  'template',
  'source',
  'status',
  'system',
  'host',
  'cpu',
  'memory',
  'disk',
] as const;

export type VMTableColumnKey =
  | (typeof defaultVMTableColumns)[number]
  | (typeof defaultTemplateTableColumns)[number];

type VMTableColumnDefinition = {
  key: VMTableColumnKey;
  header: string;
  width: number;
  headerClassName: string;
};

export function VMTable({
  vms,
  hosts,
  loading,
  selectedIds,
  actionId,
  refreshingId,
  onToggleSelected,
  onToggleAllVisible,
  onEdit,
  onRefresh,
  onMonitor,
  onConsole,
  onClone,
  onMarkTemplate,
  onUnmarkTemplate,
  onCreateFromTemplate,
  onMigrate,
  onAction,
  permissions,
  templateView,
  visibleColumns,
}: {
  vms: VirtualMachine[];
  hosts: Host[];
  loading: boolean;
  selectedIds: string[];
  actionId: string | null;
  refreshingId: string | null;
  onToggleSelected: (vm: VirtualMachine) => void;
  onToggleAllVisible: () => void;
  onEdit: (vm: VirtualMachine) => void;
  onRefresh: (vm: VirtualMachine) => void;
  onMonitor: (vm: VirtualMachine) => void;
  onConsole: (vm: VirtualMachine) => void;
  onClone: (vm: VirtualMachine) => void;
  onMarkTemplate: (vm: VirtualMachine) => void;
  onUnmarkTemplate: (vm: VirtualMachine) => void;
  onCreateFromTemplate: (vm: VirtualMachine) => void;
  onMigrate: (vm: VirtualMachine) => void;
  onAction: (vm: VirtualMachine, action: VMAction) => void;
  permissions: VMActionPermissions;
  templateView?: boolean;
  visibleColumns?: VMTableColumnKey[];
}) {
  const shouldScrollTable = vms.length > tableScrollThreshold;
  const tableScrollHeight = tableHeaderHeight + tableRowHeight * tableScrollThreshold;
  const allVisibleSelected = vms.length > 0 && vms.every(vm => selectedIds.includes(vm.id));
  const actionCount = getVisibleActionCount(permissions, Boolean(templateView));
  const columns = tableColumns(Boolean(templateView), visibleColumns);

  return (
    <div
      className="overflow-hidden rounded-xl"
      style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
    >
      <div
        data-vm-table-scroll
        className={
          (shouldScrollTable ? 'overflow-y-auto ' : '') + 'kvm-hidden-scrollbar overflow-x-auto'
        }
        style={shouldScrollTable ? { maxHeight: tableScrollHeight } : undefined}
      >
        <table className="w-full table-fixed border-separate border-spacing-0 text-center text-sm">
          <VMTableColumns
            actionCount={actionCount}
            templateView={Boolean(templateView)}
            visibleColumns={visibleColumns}
          />
          <thead>
            <tr style={{ borderBottom: '1px solid var(--kvm-border)' }}>
              <VMSelectHeaderCell shouldScroll={shouldScrollTable} onClick={onToggleAllVisible}>
                <VMSelectCheckbox
                  checked={allVisibleSelected}
                  onChange={onToggleAllVisible}
                  aria-label="选择当前虚拟机"
                />
              </VMSelectHeaderCell>
              {columns.map(column => (
                <VMHeaderCell
                  key={column.key}
                  shouldScroll={shouldScrollTable}
                  className={column.headerClassName}
                >
                  {column.header}
                </VMHeaderCell>
              ))}
              <VMHeaderCell shouldScroll={shouldScrollTable} className="px-3">
                操作
              </VMHeaderCell>
            </tr>
          </thead>
          <tbody>
            {loading && vms.length === 0 && <VMEmptyRow text="正在加载虚拟机..." />}
            {!loading && vms.length === 0 && (
              <VMEmptyRow text={templateView ? '暂无虚拟机模板' : '暂无真实虚拟机数据'} />
            )}
            {vms.map(vm => (
              <VMTableRow
                key={vm.id}
                vm={vm}
                hosts={hosts}
                selected={selectedIds.includes(vm.id)}
                actionBusy={actionId === vm.id}
                refreshing={refreshingId === vm.id}
                onToggleSelected={onToggleSelected}
                onEdit={onEdit}
                onRefresh={onRefresh}
                onMonitor={onMonitor}
                onConsole={onConsole}
                onClone={onClone}
                onMarkTemplate={onMarkTemplate}
                onUnmarkTemplate={onUnmarkTemplate}
                onCreateFromTemplate={onCreateFromTemplate}
                onMigrate={onMigrate}
                onAction={onAction}
                permissions={permissions}
                templateView={templateView}
                columns={columns}
              />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function VMTableColumns({
  actionCount,
  templateView,
  visibleColumns,
}: {
  actionCount: number;
  templateView: boolean;
  visibleColumns?: VMTableColumnKey[];
}) {
  const actionWidth = getActionColumnWidth(actionCount);
  const columns = tableColumns(templateView, visibleColumns);
  const scale =
    (100 - selectColumnWidth - actionWidth) /
    columns.reduce((total, column) => total + column.width, 0);
  const widths = columns.map(column => `${(column.width * scale).toFixed(2)}%`);

  return (
    <colgroup>
      <col style={{ width: `${selectColumnWidth}%` }} />
      {widths.map((width, index) => (
        <col key={index} style={{ width }} />
      ))}
      <col style={{ width: `${actionWidth}%` }} />
    </colgroup>
  );
}

function tableColumns(templateView: boolean, visibleColumns?: VMTableColumnKey[]) {
  const selected = new Set(
    visibleColumns ?? (templateView ? defaultTemplateTableColumns : defaultVMTableColumns)
  );
  const definitions = templateView ? templateColumnDefinitions : vmColumnDefinitions;
  return definitions.filter(column => selected.has(column.key));
}

const vmColumnDefinitions: VMTableColumnDefinition[] = [
  { key: 'identity', header: '名称 / IP', width: 15, headerClassName: 'px-4' },
  { key: 'description', header: '描述', width: 15, headerClassName: 'px-4' },
  { key: 'status', header: '状态', width: 8, headerClassName: 'px-2' },
  { key: 'system', header: '系统', width: 15, headerClassName: 'px-4' },
  { key: 'host', header: '宿主机', width: 13, headerClassName: 'px-4' },
  { key: 'cpu', header: 'CPU', width: 15, headerClassName: 'px-4' },
  { key: 'memory', header: '内存', width: 15, headerClassName: 'px-4' },
  { key: 'disk', header: '磁盘', width: 15, headerClassName: 'px-4' },
  { key: 'uptime', header: '运行时长', width: 10, headerClassName: 'px-4' },
];

const templateColumnDefinitions: VMTableColumnDefinition[] = [
  { key: 'template', header: '模板名称 / 描述', width: 15, headerClassName: 'px-4' },
  { key: 'source', header: '源虚拟机 / IP', width: 11, headerClassName: 'px-4' },
  { key: 'status', header: '状态', width: 7, headerClassName: 'px-2' },
  { key: 'system', header: '系统', width: 9, headerClassName: 'px-4' },
  { key: 'host', header: '宿主机', width: 9, headerClassName: 'px-4' },
  { key: 'cpu', header: 'CPU', width: 10, headerClassName: 'px-4' },
  { key: 'memory', header: '内存', width: 10, headerClassName: 'px-4' },
  { key: 'disk', header: '磁盘', width: 10, headerClassName: 'px-4' },
];

function getVisibleActionCount(permissions: VMActionPermissions, templateView: boolean) {
  if (templateView) {
    return 2 + (permissions.create ? 1 : 0) + (permissions.update ? 1 : 0);
  }
  return (
    2 +
    (permissions.update ? 2 : 0) +
    (permissions.console ? 1 : 0) +
    (permissions.clone ? 1 : 0) +
    (permissions.migrate ? 1 : 0) +
    (permissions.power ? 4 : 0) +
    (permissions.delete ? 1 : 0)
  );
}

function getActionColumnWidth(actionCount: number) {
  return Math.min(18, Math.max(10, 5 + actionCount * 2.1));
}

function VMHeaderCell({
  shouldScroll,
  className,
  children,
}: {
  shouldScroll: boolean;
  className: string;
  children: React.ReactNode;
}) {
  return (
    <th
      className={
        (shouldScroll ? 'sticky top-0 z-20 ' : '') +
        className +
        ' py-3 text-center align-middle text-xs font-semibold'
      }
      style={{
        background: 'var(--kvm-table-head-bg)',
        borderBottom: '1px solid var(--kvm-border)',
        color: 'var(--kvm-text-muted)',
      }}
    >
      {children}
    </th>
  );
}

function VMSelectHeaderCell({
  shouldScroll,
  onClick,
  children,
}: {
  shouldScroll: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <th
      className={
        (shouldScroll ? 'sticky top-0 z-20 ' : '') +
        'cursor-pointer px-0 py-3 text-center align-middle'
      }
      onClick={onClick}
      style={{
        background: 'var(--kvm-table-head-bg)',
        borderBottom: '1px solid var(--kvm-border)',
      }}
    >
      <div className="flex justify-center">{children}</div>
    </th>
  );
}

function VMEmptyRow({ text }: { text: string }) {
  return (
    <tr>
      <td
        colSpan={12}
        className="px-4 py-10 text-center"
        style={{ color: 'var(--kvm-text-muted)' }}
      >
        {text}
      </td>
    </tr>
  );
}

function VMTableRow({
  vm,
  hosts,
  selected,
  actionBusy,
  refreshing,
  onToggleSelected,
  onEdit,
  onRefresh,
  onMonitor,
  onConsole,
  onClone,
  onMarkTemplate,
  onUnmarkTemplate,
  onCreateFromTemplate,
  onMigrate,
  onAction,
  permissions,
  templateView,
  columns,
}: {
  vm: VirtualMachine;
  hosts: Host[];
  selected: boolean;
  actionBusy: boolean;
  refreshing: boolean;
  onToggleSelected: (vm: VirtualMachine) => void;
  onEdit: (vm: VirtualMachine) => void;
  onRefresh: (vm: VirtualMachine) => void;
  onMonitor: (vm: VirtualMachine) => void;
  onConsole: (vm: VirtualMachine) => void;
  onClone: (vm: VirtualMachine) => void;
  onMarkTemplate: (vm: VirtualMachine) => void;
  onUnmarkTemplate: (vm: VirtualMachine) => void;
  onCreateFromTemplate: (vm: VirtualMachine) => void;
  onMigrate: (vm: VirtualMachine) => void;
  onAction: (vm: VirtualMachine, action: VMAction) => void;
  permissions: VMActionPermissions;
  templateView?: boolean;
  columns: VMTableColumnDefinition[];
}) {
  const disabled = actionBusy || refreshing;
  const running = isVMRunning(vm.status);
  const paused = isVMPaused(vm.status);
  const stopped = isVMStopped(vm.status);

  return (
    <tr className="transition-colors" style={{ borderBottom: '1px solid rgba(56,78,120,0.16)' }}>
      <td
        className="cursor-pointer px-0 py-3 text-center align-middle"
        onClick={() => onToggleSelected(vm)}
      >
        <div className="flex justify-center">
          <VMSelectCheckbox
            checked={selected}
            onChange={() => onToggleSelected(vm)}
            aria-label={`选择 ${vm.name}`}
          />
        </div>
      </td>
      {columns.map(column => (
        <VMDataCell key={column.key} columnKey={column.key} vm={vm} hosts={hosts} />
      ))}
      <td className="px-3 py-3 text-center">
        <div
          className={
            templateView
              ? 'mx-auto inline-flex items-center justify-center gap-1.5'
              : 'mx-auto grid w-[198px] grid-cols-6 justify-center gap-1.5'
          }
        >
          {permissions.update && (
            <ActionButton label="编辑" disabled={disabled} onClick={() => onEdit(vm)}>
              <PencilIcon size={15} />
            </ActionButton>
          )}
          <ActionButton label="刷新" disabled={disabled} onClick={() => onRefresh(vm)}>
            <RotateCwIcon size={15} className={refreshing ? 'animate-spin' : ''} />
          </ActionButton>
          {!templateView && (
            <ActionButton label="监控" disabled={disabled} onClick={() => onMonitor(vm)}>
              <BarChart3Icon size={15} />
            </ActionButton>
          )}
          {templateView && permissions.create && (
            <ActionButton
              label="从模板创建"
              variant="clone"
              disabled={disabled || running}
              onClick={() => onCreateFromTemplate(vm)}
            >
              <Layers3Icon size={15} />
            </ActionButton>
          )}
          {templateView && permissions.update && (
            <ActionButton
              label="取消模板"
              danger
              disabled={disabled}
              onClick={() => onUnmarkTemplate(vm)}
            >
              <XCircleIcon size={15} />
            </ActionButton>
          )}
          {!templateView && permissions.update && !vm.isTemplate && (
            <ActionButton
              label="设为模板"
              variant="clone"
              disabled={disabled || running}
              onClick={() => onMarkTemplate(vm)}
            >
              <BadgeCheckIcon size={15} />
            </ActionButton>
          )}
          {!templateView && permissions.console && (
            <ActionButton
              label="控制台"
              variant="console"
              disabled={disabled}
              onClick={() => onConsole(vm)}
            >
              <MonitorIcon size={15} />
            </ActionButton>
          )}
          {!templateView && permissions.clone && (
            <ActionButton
              label="克隆"
              variant="clone"
              disabled={disabled || running}
              onClick={() => onClone(vm)}
            >
              <CopyPlusIcon size={15} />
            </ActionButton>
          )}
          {!templateView && permissions.migrate && (
            <ActionButton
              label="迁移"
              variant="clone"
              disabled={disabled || hosts.length < 2}
              onClick={() => onMigrate(vm)}
            >
              <MoveRightIcon size={15} />
            </ActionButton>
          )}
          {!templateView && permissions.power && (
            <>
              <ActionButton
                label={paused ? '恢复' : '启动'}
                disabled={disabled || running}
                onClick={() => onAction(vm, paused ? 'resume' : 'start')}
              >
                <PlayIcon size={15} />
              </ActionButton>
              <ActionButton
                label="暂停"
                disabled={disabled || !running}
                onClick={() => onAction(vm, 'pause')}
              >
                <PauseIcon size={15} />
              </ActionButton>
              <ActionButton
                label="重启"
                disabled={disabled || !running}
                onClick={() => onAction(vm, 'reboot')}
              >
                <RepeatIcon size={15} />
              </ActionButton>
              <ActionButton
                label="关机"
                disabled={disabled || stopped}
                danger
                onClick={() => onAction(vm, 'shutdown')}
              >
                <SquareIcon size={15} />
              </ActionButton>
            </>
          )}
          {!templateView && permissions.delete && (
            <ActionButton
              label="删除"
              disabled={disabled}
              danger
              onClick={() => onAction(vm, 'delete')}
            >
              <Trash2Icon size={15} />
            </ActionButton>
          )}
        </div>
      </td>
    </tr>
  );
}

function VMDataCell({
  columnKey,
  vm,
  hosts,
}: {
  columnKey: VMTableColumnKey;
  vm: VirtualMachine;
  hosts: Host[];
}) {
  if (columnKey === 'identity') {
    return (
      <td className="px-4 py-3 text-center">
        <div className="truncate font-medium" style={{ color: 'var(--kvm-text)' }}>
          {vm.name}
        </div>
        <div className="truncate font-mono text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
          {vm.primaryIp || '-'}
        </div>
      </td>
    );
  }
  if (columnKey === 'template') {
    return (
      <td className="px-4 py-3 text-center text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
        <div className="min-w-0">
          <div className="truncate font-medium" style={{ color: 'var(--kvm-text)' }}>
            {vm.templateName || vm.name}
          </div>
          <div className="mt-0.5 truncate">{vm.templateDescription || '-'}</div>
        </div>
      </td>
    );
  }
  if (columnKey === 'source') {
    return (
      <td className="px-4 py-3 text-center">
        <div className="truncate font-medium" style={{ color: 'var(--kvm-text)' }}>
          {vm.name}
        </div>
        <div className="truncate font-mono text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
          {vm.primaryIp || '-'}
        </div>
      </td>
    );
  }
  if (columnKey === 'description') {
    return (
      <td
        className="px-4 py-3 text-center text-xs align-middle"
        style={{ color: 'var(--kvm-text-muted)' }}
      >
        <div className="line-clamp-2 leading-5">{vmDescription(vm)}</div>
      </td>
    );
  }
  if (columnKey === 'status') {
    return (
      <td className="px-2 py-3 text-center">
        <div className="flex justify-center">
          <StatusBadge status={vm.status} />
        </div>
      </td>
    );
  }
  if (columnKey === 'system') {
    return (
      <td className="px-4 py-3 text-center text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
        {formatOSType(vm.osType, vm.name)}
      </td>
    );
  }
  if (columnKey === 'host') {
    return (
      <td
        className="px-4 py-3 text-center font-mono text-xs"
        style={{ color: 'var(--kvm-text-muted)' }}
      >
        {hostAddress(vm, hosts)}
      </td>
    );
  }
  if (columnKey === 'cpu') {
    return (
      <td className="min-w-36 px-4 py-3 text-center">
        <VMMetricCell
          value={vm.cpuUsage}
          available={vm.cpuUsageAvailable}
          prefix={`${vm.cpuCores} vCPU`}
        />
      </td>
    );
  }
  if (columnKey === 'memory') {
    return (
      <td className="min-w-36 px-4 py-3 text-center">
        <VMMetricCell
          value={vm.memoryUsage}
          available={vm.memoryUsageAvailable}
          prefix={formatMemoryBytes(vm.memoryBytes)}
        />
      </td>
    );
  }
  if (columnKey === 'disk') {
    return (
      <td className="min-w-36 px-4 py-3 text-center">
        <DiskMetricCell vm={vm} />
      </td>
    );
  }
  return (
    <td className="px-4 py-3 text-center text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
      <span className="font-mono">{formatUptime(vm.uptimeSeconds)}</span>
    </td>
  );
}

function hostAddress(vm: VirtualMachine, hosts: Host[]) {
  const host = hosts.find(item => item.id === vm.hostId);
  return host?.address || '-';
}

function vmDescription(vm: VirtualMachine) {
  return vm.templateDescription || vm.description || '-';
}

export type VMActionPermissions = {
  create: boolean;
  update: boolean;
  power: boolean;
  delete: boolean;
  force: boolean;
  console: boolean;
  clone: boolean;
  migrate: boolean;
};

function VMSelectCheckbox({
  checked,
  onChange,
  'aria-label': ariaLabel,
}: {
  checked: boolean;
  onChange: () => void;
  'aria-label': string;
}) {
  return (
    <label
      className="group inline-flex h-7 w-7 cursor-pointer items-center justify-center rounded-lg"
      onClick={event => event.stopPropagation()}
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={onChange}
        aria-label={ariaLabel}
        className="peer sr-only"
      />
      <span
        className="flex h-5 w-5 items-center justify-center rounded-md border transition-all duration-150 peer-focus-visible:ring-2 peer-focus-visible:ring-blue-300 peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-transparent group-hover:-translate-y-0.5 group-active:translate-y-0"
        style={{
          background: checked
            ? 'linear-gradient(135deg, #3b82f6, #06b6d4)'
            : 'var(--kvm-control-bg)',
          borderColor: checked ? 'rgba(147,197,253,0.72)' : 'var(--kvm-border)',
          boxShadow: checked
            ? '0 8px 18px rgba(59,130,246,0.22), inset 0 1px 0 rgba(255,255,255,0.22)'
            : 'inset 0 1px 0 rgba(255,255,255,0.06)',
        }}
      >
        <CheckIcon
          size={14}
          className={
            checked ? 'scale-100 opacity-100 transition-all' : 'scale-75 opacity-0 transition-all'
          }
          color="#ffffff"
          strokeWidth={3}
        />
      </span>
    </label>
  );
}
