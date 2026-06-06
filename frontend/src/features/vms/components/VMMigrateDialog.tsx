import { useEffect, useMemo, useState } from 'react';
import { CheckCircleIcon, CircleDashedIcon, KeyRoundIcon, MoveRightIcon, ShieldCheckIcon, XCircleIcon, XIcon } from 'lucide-react';
import { toast } from 'sonner';

import {
  precheckVMMigration,
  type Host,
  type VirtualMachine,
  type VMMigrationPrecheckReport,
  type VMMigratePayload,
} from '../../../lib/api';
import { formatBytes } from '../../../lib/format';
import { DialogPortal } from '../../../components/kvm/DialogPortal';
import { SelectMenu } from '../../../components/kvm/SelectMenu';
import { KvmTooltip } from '../../../components/kvm/StatusBadge';
import { fieldStyle, inputClass, InlineNotice } from './edit/EditShared';
import { CheckToggle, PrimaryButton } from './VMEditControls';
import { MigrationHostnameDialog, MigrationSSHKeyDialog } from './VMMigrationFixDialogs';
import { runVMMigrate } from '../utils/runVMMigrate';
import { defaultMigrationMode, isVMRunning, vmStatusLabel } from '../utils/vmStatus';

export function VMMigrateDialog({
  vm,
  hosts,
  onClose,
}: {
  vm: VirtualMachine;
  hosts: Host[];
  onClose: () => void;
}) {
  const targetHosts = hosts.filter(host => host.id !== vm.hostId);
  const [targetAgentId, setTargetAgentId] = useState(targetHosts[0]?.id || '');
  const [mode, setMode] = useState(defaultMigrationMode(vm.status));
  const [copyDisks, setCopyDisks] = useState(true);
  const [persistent, setPersistent] = useState(true);
  const [undefineSource, setUndefineSource] = useState(true);
  const [autoConverge, setAutoConverge] = useState(true);
  const [postCopy, setPostCopy] = useState(false);
  const [destinationUri, setDestinationUri] = useState('');
  const [customUri, setCustomUri] = useState(false);
  const [busy, setBusy] = useState(false);
  const [prechecking, setPrechecking] = useState(false);
  const [precheckReport, setPrecheckReport] = useState<VMMigrationPrecheckReport | null>(null);
  const [precheckPayloadKey, setPrecheckPayloadKey] = useState('');
  const [precheckPayload, setPrecheckPayload] = useState<VMMigratePayload | null>(null);
  const [sshDialog, setSSHDialog] = useState<{ payload: VMMigratePayload; message: string } | null>(null);
  const [hostnameDialog, setHostnameDialog] = useState<{ payload: VMMigratePayload; message: string } | null>(null);

  const hostOptions = useMemo(
    () => targetHosts.map(host => ({ value: host.id, label: host.address || host.hostname || host.name || host.id, tooltip: host.name || host.hostname || host.id })),
    [targetHosts]
  );
  const selectedHost = targetHosts.find(host => host.id === targetAgentId);
  const defaultUri = selectedHost ? `qemu+ssh://${hostForURI(selectedHost)}/system` : '';
  const live = mode === 'live';
  const running = isVMRunning(vm.status);
  const sourceHost = hosts.find(host => host.id === vm.hostId);
  const sourceCleanupLabel = copyDisks ? '迁移后清理源虚拟机' : '迁移后取消源定义';
  const sourceCleanupDescription = copyDisks ? '迁移成功后删除源定义和源普通磁盘' : '避免源宿主机残留重复定义';
  const migrationPayloadKey = useMemo(
    () =>
      JSON.stringify({
        targetAgentId,
        destinationUri: customUri ? destinationUri.trim() : defaultUri,
        live,
        copyDisks,
        persistent,
        undefineSource,
        autoConverge: live && autoConverge,
        postCopy: live && postCopy,
        targetStatus: selectedHost?.status || '',
      }),
    [
      autoConverge,
      copyDisks,
      customUri,
      defaultUri,
      destinationUri,
      live,
      persistent,
      postCopy,
      selectedHost?.status,
      targetAgentId,
      undefineSource,
    ]
  );
  const precheckPassed = Boolean(precheckReport?.passed && precheckPayloadKey === migrationPayloadKey);
  const migrationSubmitting = busy && !sshDialog;
  const controlsLocked = busy || prechecking;
  const migrateDisabledReason = migrationDisabledReason({
    busy: migrationSubmitting,
    prechecking,
    precheckReport,
    precheckPassed,
  });
  const migrationModeOptions = [
    { value: 'live', label: '热迁移', disabled: !running, tooltip: running ? '迁移运行中的虚拟机' : '运行中虚拟机可选择热迁移' },
    { value: 'cold', label: '冷迁移', disabled: running, tooltip: running ? '运行中虚拟机请使用热迁移' : '已停止虚拟机可选择冷迁移' },
  ];

  useEffect(() => {
    if (!precheckReport) return;
    if (precheckPayloadKey === migrationPayloadKey) return;
    setPrecheckReport(null);
    setPrecheckPayloadKey('');
    setPrecheckPayload(null);
  }, [migrationPayloadKey, precheckPayloadKey, precheckReport]);

  async function submit() {
    if (!precheckPassed || !precheckPayload) {
      toast.warning('预检通过后再执行迁移');
      return;
    }
    setBusy(true);
    try {
      await runVMMigrate({
        vmId: vm.id,
        vmName: vm.name,
        payload: precheckPayload,
        onQueued: onClose,
        onSSHPasswordRequired: message => setSSHDialog({ payload: precheckPayload, message }),
      });
    } finally {
      setBusy(false);
    }
  }

  async function runPrecheck() {
    const payload = buildMigrationPayload();
    if (!payload) return;
    await runPrecheckWithPayload(payload);
  }

  async function runPrecheckWithPayload(payload: VMMigratePayload) {
    setPrechecking(true);
    try {
      const report = await precheckVMMigration(vm.id, payload);
      setPrecheckReport(report);
      setPrecheckPayloadKey(migrationPayloadKey);
      setPrecheckPayload(payload);
      if (report.passed) toast.success('迁移预检通过');
      else toast.warning('迁移预检未通过');
    } catch (error) {
      setPrecheckReport(null);
      setPrecheckPayloadKey('');
      setPrecheckPayload(null);
      toast.error(error instanceof Error ? error.message : '迁移预检失败');
    } finally {
      setPrechecking(false);
    }
  }

  function buildMigrationPayload() {
    const warning = migrationWarning({
      targetAgentId,
      selectedHost,
      vm,
      live,
      running,
      customUri,
      destinationUri,
      defaultUri,
    });
    if (warning) {
      toast.warning(warning);
      return null;
    }
    return {
      targetAgentId,
      destinationUri: customUri ? destinationUri.trim() : defaultUri,
      live,
      copyDisks,
      persistent,
      undefineSource,
      autoConverge: live && autoConverge,
      postCopy: live && postCopy,
    };
  }

  return (
    <DialogPortal>
    <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-3 py-5">
      <div className="kvm-dialog-panel flex max-h-[88vh] w-[min(92vw,900px)] flex-col overflow-hidden rounded-2xl shadow-2xl">
        <header className="flex min-h-14 items-center justify-between border-b px-4 py-2.5" style={{ borderColor: 'var(--kvm-border)' }}>
          <div className="flex items-center gap-2">
            <MoveRightIcon size={17} style={{ color: '#93c5fd' }} />
            <h2 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>迁移虚拟机</h2>
          </div>
          <button type="button" onClick={onClose} disabled={controlsLocked} className="kvm-action-button flex h-8 w-8 items-center justify-center rounded-lg border disabled:cursor-not-allowed disabled:opacity-60" style={{ background: 'var(--kvm-control-bg)', borderColor: 'var(--kvm-border)', color: 'var(--kvm-text-muted)' }} aria-label="关闭迁移窗口">
            <XIcon size={15} />
            </button>
          </header>
          <main className="kvm-hidden-scrollbar flex-1 overflow-y-auto p-4" style={{ background: 'var(--kvm-control-bg-soft)' }}>
            <div className="kvm-dialog-card space-y-4 rounded-2xl p-4">
              <InlineNotice tone="warning">迁移前请确认目标宿主机具备兼容 CPU、同名网络桥接和可访问存储</InlineNotice>
              <Panel title="迁移对象">
              <div className="grid gap-3 md:grid-cols-5">
                <Metric label="虚拟机" value={vm.name} />
                <Metric label="源宿主机" value={sourceHostLabel(sourceHost, vm)} />
                <Metric label="状态" value={vmStatusLabel(vm.status)} />
                <Metric label="规格" value={`${vm.cpuCores} vCPU / ${formatBytes(vm.memoryBytes, 'GB')}`} />
                <Metric label="磁盘" value={`${vm.disks.length || 1} 块 / ${formatBytes(vm.diskBytes, 'GB')}`} />
              </div>
            </Panel>
            <Panel title="目标与方式">
              <div className="grid gap-3 lg:grid-cols-3">
                <Field label="目标宿主机">
                  <SelectMenu value={targetAgentId} options={hostOptions} placeholder="选择目标宿主机" disabled={controlsLocked} onChange={setTargetAgentId} />
                </Field>
                <Field label="迁移类型">
                  <SelectMenu value={mode} options={migrationModeOptions} placeholder="迁移类型" disabled={controlsLocked} onChange={setMode} />
                </Field>
                <Field label="迁移 URI">
                  <input
                    value={customUri ? destinationUri : defaultUri}
                    disabled={controlsLocked || !customUri}
                    onChange={event => setDestinationUri(event.target.value)}
                    placeholder={defaultUri}
                    className={inputClass}
                    style={fieldStyle}
                  />
                  <div className="mt-3">
                    <CheckToggle checked={customUri} disabled={controlsLocked} onChange={setCustomUri} label="自定义迁移 URI" />
                  </div>
                </Field>
              </div>
            </Panel>
            <Panel title="迁移策略">
              <div className="grid gap-3 sm:grid-cols-3">
                <OptionToggle label="复制本地磁盘" description="本地磁盘迁移时启用，共享存储无需启用" checked={copyDisks} disabled={controlsLocked} onChange={setCopyDisks} />
                <OptionToggle label="持久化目标定义" description="迁移后在目标宿主机保留定义" checked={persistent} disabled={controlsLocked} onChange={setPersistent} />
                <OptionToggle label={sourceCleanupLabel} description={sourceCleanupDescription} checked={undefineSource} disabled={controlsLocked} onChange={setUndefineSource} />
              </div>
            </Panel>
            <Panel title="热迁移优化">
              <div className="grid gap-3 sm:grid-cols-2">
                <OptionToggle label="自动收敛" description="降低迁移期间写入速度以便收敛" checked={autoConverge} disabled={controlsLocked || !live} onChange={setAutoConverge} />
                <OptionToggle label="Post-copy" description="降低收敛等待，但失败恢复风险更高" checked={postCopy} disabled={controlsLocked || !live} onChange={setPostCopy} />
              </div>
              {!live && <div className="mt-3 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>冷迁移不会使用自动收敛和 Post-copy 参数</div>}
            </Panel>
            {precheckReport && (
              <PrecheckPanel
                report={precheckReport}
                disabled={controlsLocked}
                onSetupSSHKey={item => precheckPayload && setSSHDialog({ payload: precheckPayload, message: item.message })}
                onSetupHostname={item => precheckPayload && setHostnameDialog({ payload: precheckPayload, message: item.message })}
              />
            )}
            <div className="flex flex-wrap justify-end gap-2">
              <SecondaryButton label={prechecking ? '预检中' : '预检'} disabled={controlsLocked} onClick={() => void runPrecheck()} />
              <KvmTooltip label={migrateDisabledReason} placement="top" align="end">
                <span className="inline-flex">
                  <PrimaryButton label={migrationSubmitting ? '提交中' : '迁移'} disabled={controlsLocked || !precheckPassed} onClick={() => void submit()} />
                </span>
              </KvmTooltip>
            </div>
          </div>
        </main>
      </div>
    </div>
    {sshDialog && (
      <MigrationSSHKeyDialog
        vmId={vm.id}
        vmName={vm.name}
        payload={sshDialog.payload}
        message={sshDialog.message}
        onClose={() => setSSHDialog(null)}
        onConfigured={() => {
          const retryPayload = sshDialog.payload;
          setSSHDialog(null);
          void runPrecheckWithPayload(retryPayload);
        }}
      />
    )}
    {hostnameDialog && (
      <MigrationHostnameDialog
        vmId={vm.id}
        vmName={vm.name}
        payload={hostnameDialog.payload}
        message={hostnameDialog.message}
        onClose={() => setHostnameDialog(null)}
        onConfigured={() => {
          const retryPayload = hostnameDialog.payload;
          setHostnameDialog(null);
          void runPrecheckWithPayload(retryPayload);
        }}
      />
    )}
    </DialogPortal>
  );
}

function migrationDisabledReason({
  busy,
  prechecking,
  precheckReport,
  precheckPassed,
}: {
  busy: boolean;
  prechecking: boolean;
  precheckReport: VMMigrationPrecheckReport | null;
  precheckPassed: boolean;
}) {
  if (busy || precheckPassed) return undefined;
  if (prechecking) return '预检中，请稍候';
  if (precheckReport && !precheckReport.passed) return '预检未通过，请处理失败项后重新预检';
  return '预检通过后再执行迁移';
}

function PrecheckPanel({
  report,
  disabled,
  onSetupSSHKey,
  onSetupHostname,
}: {
  report: VMMigrationPrecheckReport;
  disabled: boolean;
  onSetupSSHKey: (item: VMMigrationPrecheckReport['items'][number]) => void;
  onSetupHostname: (item: VMMigrationPrecheckReport['items'][number]) => void;
}) {
  return (
    <Panel title="迁移预检">
      <div className="grid gap-2 md:grid-cols-2">
        {report.items.map(item => (
          <div key={item.key} className="flex gap-2 rounded-lg border p-3" style={{ background: 'var(--kvm-control-bg)', borderColor: 'var(--kvm-border)' }}>
            <PrecheckIcon status={item.status} />
            <div className="min-w-0 flex-1">
              <div className="flex items-center justify-between gap-2">
                <div className="text-xs font-semibold" style={{ color: 'var(--kvm-text)' }}>{item.label}</div>
                {canSetupMigrationSSHKey(item) && (
                  <KvmTooltip label="输入目标宿主机 SSH 密码并配置免密" placement="top" align="end">
                    <button
                      type="button"
                      disabled={disabled}
                      onClick={() => onSetupSSHKey(item)}
                      className="kvm-action-button inline-flex h-7 shrink-0 items-center gap-1 rounded-lg border px-2 text-xs font-semibold disabled:cursor-not-allowed disabled:opacity-60"
                      style={{ background: 'var(--kvm-control-bg-soft)', borderColor: 'var(--kvm-border)', color: 'var(--kvm-text)' }}
                      aria-label="配置迁移 SSH 免密"
                    >
                      <KeyRoundIcon size={13} />
                      配置免密
                    </button>
                  </KvmTooltip>
                )}
                {canSetupMigrationHostname(item) && (
                  <KvmTooltip label="设置目标主机名并写入源目标 hosts 解析" placement="top" align="end">
                    <button
                      type="button"
                      disabled={disabled}
                      onClick={() => onSetupHostname(item)}
                      className="kvm-action-button inline-flex h-7 shrink-0 items-center gap-1 rounded-lg border px-2 text-xs font-semibold disabled:cursor-not-allowed disabled:opacity-60"
                      style={{ background: 'var(--kvm-control-bg-soft)', borderColor: 'var(--kvm-border)', color: 'var(--kvm-text)' }}
                      aria-label="修复迁移目标主机名"
                    >
                      <KeyRoundIcon size={13} />
                      修复主机名
                    </button>
                  </KvmTooltip>
                )}
              </div>
              <div className="mt-1 text-xs leading-5" style={{ color: 'var(--kvm-text-muted)' }}>{item.message}</div>
            </div>
          </div>
        ))}
      </div>
    </Panel>
  );
}

function canSetupMigrationSSHKey(item: VMMigrationPrecheckReport['items'][number]) {
  return item.key === 'migration-channel' && item.status === 'failed' && item.code === 'vm_migrate_ssh_password_required';
}

function canSetupMigrationHostname(item: VMMigrationPrecheckReport['items'][number]) {
  return item.key === 'migration-channel' && item.status === 'failed' && item.code === 'vm_migrate_target_hostname_localhost';
}

function PrecheckIcon({ status }: { status: VMMigrationPrecheckReport['items'][number]['status'] }) {
  if (status === 'passed') return <CheckCircleIcon size={15} className="mt-0.5 shrink-0" style={{ color: 'var(--kvm-status-green-text)' }} />;
  if (status === 'failed') return <XCircleIcon size={15} className="mt-0.5 shrink-0" style={{ color: 'var(--kvm-status-red-text)' }} />;
  return <CircleDashedIcon size={15} className="mt-0.5 shrink-0" style={{ color: 'var(--kvm-text-muted)' }} />;
}

function SecondaryButton({ label, disabled, onClick }: { label: string; disabled?: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className="kvm-action-button inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60"
      style={{ background: 'var(--kvm-control-bg)', borderColor: 'var(--kvm-border)', color: 'var(--kvm-text)' }}
    >
      <ShieldCheckIcon size={15} />
      {label}
    </button>
  );
}

function migrationWarning({
  targetAgentId,
  selectedHost,
  vm,
  live,
  running,
  customUri,
  destinationUri,
  defaultUri,
}: {
  targetAgentId: string;
  selectedHost?: Host;
  vm: VirtualMachine;
  live: boolean;
  running: boolean;
  customUri: boolean;
  destinationUri: string;
  defaultUri: string;
}) {
  if (!targetAgentId || !selectedHost) return '请选择目标宿主机';
  if (targetAgentId === vm.hostId) return '目标宿主机不能与源宿主机相同';
  if (selectedHost.status !== 'online') return '目标宿主机当前不是在线状态';
  if (!live && running) return '运行中的虚拟机请选择热迁移';
  if (live && !running) return '非运行状态的虚拟机请选择冷迁移';
  if (selectedHost.cpuCores > 0 && vm.cpuCores > selectedHost.cpuCores) {
    return '目标宿主机逻辑 CPU 不足';
  }
  if (selectedHost.memoryBytes > 0 && vm.memoryBytes > selectedHost.memoryBytes) {
    return '目标宿主机内存不足';
  }
  if (customUri && !destinationUri.trim()) return '请输入迁移 URI';
  if (!customUri && !defaultUri.trim()) return '目标宿主机迁移 URI 为空';
  return '';
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label><div className="mb-1.5 text-xs font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>{label}</div>{children}</label>;
}

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return <section className="rounded-xl border p-4" style={{ background: 'var(--kvm-control-bg-soft)', borderColor: 'var(--kvm-border)' }}><h3 className="mb-3 text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>{title}</h3>{children}</section>;
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="kvm-card-hover rounded-lg border px-3 py-2" style={{ background: 'var(--kvm-control-bg)', borderColor: 'var(--kvm-border)' }}>
      <div className="text-xs font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>{label}</div>
      <div className="mt-1 truncate text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>{value}</div>
    </div>
  );
}

function OptionToggle({
  label,
  description,
  checked,
  disabled,
  onChange,
}: {
  label: string;
  description: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <div className="kvm-card-hover rounded-lg border p-3" style={{ background: 'var(--kvm-control-bg)', borderColor: 'var(--kvm-border)' }}>
      <CheckToggle checked={checked} disabled={disabled} onChange={onChange} label={label} />
      <div className="mt-2 text-xs leading-5" style={{ color: 'var(--kvm-text-muted)' }}>{description}</div>
    </div>
  );
}

function hostForURI(host: Host) {
  return host.address || host.hostname || host.name || host.id;
}

function sourceHostLabel(host: Host | undefined, vm: VirtualMachine) {
  return host?.address || host?.hostname || host?.name || vm.hostName || vm.hostId || '-';
}
