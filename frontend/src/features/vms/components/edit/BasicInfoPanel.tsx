import { useEffect, useState } from 'react';

import { EyeIcon, EyeOffIcon, PowerIcon } from 'lucide-react';
import { toast } from 'sonner';

import {
  renameVM,
  updateVMConfig,
  updateVMConsole,
  updateVMAutostart,
  type VMConfig,
  type VirtualMachine,
} from '../../../../lib/api';
import { PrimaryButton } from '../VMEditControls';
import { CardSection, FieldText, fieldStyle, FormGrid, InlineNotice, inputClass } from './EditShared';
import { bytesToMB } from './editUtils';
import { isVMRunning } from '../../utils/vmStatus';

export function BasicInfoPanel({
  vm,
  config,
  onConfigChange,
}: {
  vm: VirtualMachine;
  config: VMConfig | null;
  onConfigChange: (config: VMConfig) => void;
}) {
  const [name, setName] = useState(config?.name || vm.name);
  const [description, setDescription] = useState(config?.description || '');
  const [autostart, setAutostart] = useState(Boolean(config?.autostart));
  const [consolePasswordEnabled, setConsolePasswordEnabled] = useState(Boolean(config?.graphics?.passwordEnabled));
  const [consolePassword, setConsolePassword] = useState('');
  const [editingConsolePassword, setEditingConsolePassword] = useState(false);
  const [consolePasswordVisible, setConsolePasswordVisible] = useState(false);
  const [saving, setSaving] = useState(false);
  const [savingAutostart, setSavingAutostart] = useState(false);
  const running = isVMRunning(config?.status || vm.status);
  const nameChanged = Boolean(config && name.trim() !== (config.name || vm.name));
  const descriptionChanged = Boolean(config && description !== (config.description || ''));
  const currentConsolePasswordEnabled = Boolean(config?.graphics?.passwordEnabled);
  const disableConsolePasswordBlocked = running && currentConsolePasswordEnabled;
  const consolePasswordChanged = Boolean(config && (
    consolePasswordEnabled !== currentConsolePasswordEnabled ||
    (consolePasswordEnabled && consolePassword.trim() !== '')
  ));

  useEffect(() => {
    if (!config) return;
    setName(config.name || vm.name);
    setDescription(config.description || '');
    setAutostart(config.autostart);
    setConsolePasswordEnabled(Boolean(config.graphics?.passwordEnabled));
    setConsolePassword('');
    setEditingConsolePassword(false);
    setConsolePasswordVisible(false);
  }, [config, vm.name]);

  async function handleSubmit() {
    if (!config) return toast.warning('虚拟机配置尚未加载完成');
    const nextName = name.trim();
    if (!nextName) return toast.error('虚拟机名称不能为空');
    if (running && nameChanged) return toast.error('虚拟机正在运行，无法修改名称');
    if (disableConsolePasswordBlocked && !consolePasswordEnabled)
      return toast.error('运行中的虚拟机不支持关闭控制台密码，请先关闭虚拟机后再操作');
    if (consolePasswordEnabled && consolePasswordChanged && !consolePassword.trim()) return toast.warning('请输入控制台密码');
    if (!nameChanged && !descriptionChanged && !consolePasswordChanged) return toast.warning('请先修改配置');

    setSaving(true);
    try {
      let currentConfig = config;
      let targetId = vm.id;
      if (nameChanged) {
        const renamed = await renameVM(vm.id, { name: nextName });
        currentConfig = renamed.config;
        targetId = renamed.config.uuid ? vm.id : `${vm.hostId}:${renamed.config.name}`;
        onConfigChange(renamed.config);
      }
      if (nameChanged || descriptionChanged) {
        const result = await updateVMConfig(targetId, {
          description,
          currentCpu: currentConfig.currentCpu,
          maximumCpu: currentConfig.maximumCpu,
          currentMemoryMB: bytesToMB(currentConfig.currentMemoryBytes),
          maximumMemoryMB: bytesToMB(currentConfig.maximumMemoryBytes),
          memoryStatsPeriod: currentConfig.memoryStatsPeriod || 0,
        });
        currentConfig = result.config;
        onConfigChange(result.config);
      }
      if (consolePasswordChanged) {
        const result = await updateVMConsole(targetId, {
          passwordEnabled: consolePasswordEnabled,
          password: consolePasswordEnabled ? consolePassword.trim() : '',
        });
        currentConfig = {
          ...result.config,
          graphics: {
            ...result.config.graphics,
            passwordEnabled: consolePasswordEnabled,
          },
        };
        setConsolePasswordEnabled(consolePasswordEnabled);
        onConfigChange(currentConfig);
        setConsolePassword('');
        setEditingConsolePassword(false);
      }
      toast.success('虚拟机配置已修改');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '虚拟机配置修改失败');
    } finally {
      setSaving(false);
    }
  }

  async function handleAutostartChange(nextAutostart: boolean) {
    if (nextAutostart === autostart || savingAutostart) return;
    const previous = autostart;
    setAutostart(nextAutostart);
    setSavingAutostart(true);
    try {
      await updateVMAutostart(vm.id, nextAutostart);
      onConfigChange({
        ...(config || fallbackConfig(vm)),
        autostart: nextAutostart,
      });
      toast.success(nextAutostart ? '虚拟机自启动已启用' : '虚拟机自启动已关闭');
    } catch (error) {
      setAutostart(previous);
      toast.error(error instanceof Error ? error.message : '虚拟机自启动修改失败');
    } finally {
      setSavingAutostart(false);
    }
  }

  return (
    <section className="mx-auto max-w-3xl space-y-4">
      <CardSection title="基本配置">
        {running && (
          <div
            className="mb-3 rounded-lg border px-3 py-2 text-xs"
            style={{
              background: 'rgba(245,158,11,0.08)',
              borderColor: 'rgba(245,158,11,0.28)',
              color: '#fbbf24',
            }}
          >
            虚拟机正在运行，无法修改名称
          </div>
        )}
        <FormGrid>
          <FieldText>名称</FieldText>
          <input
            value={name}
            onChange={event => setName(event.target.value)}
            disabled={running}
            placeholder={vm.name}
            className={inputClass + ' disabled:cursor-not-allowed disabled:opacity-60'}
            style={fieldStyle}
          />
          <FieldText>描述</FieldText>
          <input
            value={description}
            onChange={event => setDescription(event.target.value)}
            placeholder="None"
            className={inputClass}
            style={fieldStyle}
          />
          <FieldText>随宿主机同启</FieldText>
          <div className="flex items-center gap-2">
            <button
              type="button"
              disabled={savingAutostart}
              onClick={() => void handleAutostartChange(!autostart)}
              className="kvm-action-button inline-flex h-9 w-fit items-center gap-2 rounded-lg border px-4 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60"
              style={{
                background: autostart ? 'rgba(16,185,129,0.12)' : 'var(--kvm-control-bg)',
                borderColor: autostart ? 'rgba(16,185,129,0.38)' : 'var(--kvm-border)',
                color: autostart ? '#86efac' : 'var(--kvm-text)',
              }}
            >
              <PowerIcon size={14} />
              {savingAutostart ? '保存中...' : autostart ? '已启用' : '启用'}
            </button>
          </div>
        </FormGrid>
      </CardSection>

      <CardSection title="控制台配置">
        {running && (
          <div
            className="mb-3 rounded-lg border px-3 py-2 text-xs"
            style={{
              background: 'rgba(245,158,11,0.08)',
              borderColor: 'rgba(245,158,11,0.28)',
              color: '#fbbf24',
            }}
          >
            虚拟机正在运行，可启用或修改控制台密码，不支持关闭已启用的控制台密码
          </div>
        )}
        <FormGrid>
          <FieldText>控制台类型</FieldText>
          <input value={(config?.graphics?.type || 'vnc').toUpperCase()} disabled className={inputClass + ' disabled:cursor-not-allowed disabled:opacity-60'} style={fieldStyle} />
          <FieldText>密码状态</FieldText>
          <button
            type="button"
            disabled={saving || disableConsolePasswordBlocked}
            onClick={() => {
              if (disableConsolePasswordBlocked) {
                toast.warning('运行中的虚拟机不支持关闭控制台密码，请先关闭虚拟机后再操作');
                return;
              }
              setConsolePasswordEnabled(value => !value);
            }}
            className="kvm-action-button inline-flex h-9 w-fit items-center rounded-lg border px-4 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60"
            style={{
              background: consolePasswordEnabled ? 'rgba(16,185,129,0.12)' : 'var(--kvm-control-bg)',
              borderColor: consolePasswordEnabled ? 'rgba(16,185,129,0.38)' : 'var(--kvm-border)',
              color: consolePasswordEnabled ? '#86efac' : 'var(--kvm-text)',
            }}
          >
            {consolePasswordEnabled ? '已启用' : '未启用'}
          </button>
          {consolePasswordEnabled && (
            <>
              <FieldText>新密码</FieldText>
              {currentConsolePasswordEnabled && !editingConsolePassword ? (
                <button
                  type="button"
                  disabled={saving}
                  onClick={() => setEditingConsolePassword(true)}
                  className="kvm-action-button inline-flex h-9 w-fit items-center rounded-lg border px-4 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60"
                  style={{
                    background: 'var(--kvm-control-bg)',
                    borderColor: 'var(--kvm-border)',
                    color: 'var(--kvm-text)',
                  }}
                >
                  修改
                </button>
              ) : (
                <PasswordInput
                  value={consolePassword}
                  disabled={saving}
                  visible={consolePasswordVisible}
                  onVisibleChange={setConsolePasswordVisible}
                  onChange={setConsolePassword}
                />
              )}
            </>
          )}
        </FormGrid>
      </CardSection>

      <div className="flex justify-end">
        <PrimaryButton
          label={saving ? '修改中...' : '修改'}
          disabled={saving}
          onClick={() => void handleSubmit()}
        />
      </div>
    </section>
  );
}

function PasswordInput({
  value,
  disabled,
  visible,
  onVisibleChange,
  onChange,
}: {
  value: string;
  disabled?: boolean;
  visible: boolean;
  onVisibleChange: (visible: boolean) => void;
  onChange: (value: string) => void;
}) {
  const Icon = visible ? EyeOffIcon : EyeIcon;
  return (
    <div className="relative">
      <input
        value={value}
        disabled={disabled}
        type={visible ? 'text' : 'password'}
        autoComplete="new-password"
        onChange={event => onChange(event.target.value)}
        className={inputClass + ' pr-10'}
        style={fieldStyle}
      />
      <button
        type="button"
        disabled={disabled}
        aria-label={visible ? '隐藏控制台密码' : '显示控制台密码'}
        onClick={() => onVisibleChange(!visible)}
        className="kvm-action-button absolute right-1.5 top-1/2 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-md disabled:cursor-not-allowed disabled:opacity-50"
        style={{ color: 'var(--kvm-text-muted)' }}
      >
        <Icon size={15} />
      </button>
    </div>
  );
}

function fallbackConfig(vm: VirtualMachine): VMConfig {
  return {
    name: vm.name,
    uuid: vm.uuid,
    osType: vm.osType,
    status: vm.status,
    description: '',
    autostart: false,
    currentCpu: vm.cpuCores,
    maximumCpu: vm.cpuCores,
    hostCpu: vm.cpuCores,
    currentMemoryBytes: vm.memoryBytes,
    maximumMemoryBytes: vm.memoryBytes,
    hostMemoryBytes: vm.memoryBytes,
    memoryStatsPeriod: 0,
    disks: [],
    interfaces: [],
    cdroms: [],
    graphics: { type: '', listen: '', port: '', passwordEnabled: false },
    xml: '',
  };
}
