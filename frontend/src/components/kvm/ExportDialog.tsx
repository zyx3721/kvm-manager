import { useEffect, useMemo, useState } from 'react';
import { CheckIcon, DownloadIcon, FileDownIcon, Loader2Icon, XIcon } from 'lucide-react';
import { toast } from 'sonner';

import { DialogPortal } from './DialogPortal';
import { SelectMenu } from './SelectMenu';
import {
  exportRows,
  localTimestamp,
  sanitizeExportFileName,
  type ExportColumn,
  type ExportFormat,
} from '../../lib/exportData';

const formatOptions: Array<{ value: ExportFormat; label: string }> = [
  { value: 'xlsx', label: 'XLSX' },
  { value: 'xls', label: 'XLS' },
  { value: 'csv', label: 'CSV' },
  { value: 'txt', label: 'TXT' },
];

const defaultFormat: ExportFormat = 'xlsx';

export function ExportDialog<T>({
  open,
  title,
  defaultName,
  rows,
  columns,
  loading = false,
  loadingText = '正在读取导出数据',
  onClose,
}: {
  open: boolean;
  title: string;
  defaultName: string;
  rows: T[];
  columns: ExportColumn<T>[];
  loading?: boolean;
  loadingText?: string;
  onClose: () => void;
}) {
  const [format, setFormat] = useState<ExportFormat>(defaultFormat);
  const [fileName, setFileName] = useState(defaultName);
  const [selectedColumnIds, setSelectedColumnIds] = useState<string[]>(() =>
    columns.map(column => columnId(column))
  );
  const previewName = useMemo(
    () => `${sanitizeExportFileName(fileName || defaultName)}.${format}`,
    [defaultName, fileName, format]
  );

  useEffect(() => {
    if (open) {
      setFormat(defaultFormat);
      setFileName(defaultName);
      setSelectedColumnIds(columns.map(column => columnId(column)));
    }
  }, [columns, defaultName, open]);

  const selectedColumns = useMemo(
    () => columns.filter(column => selectedColumnIds.includes(columnId(column))),
    [columns, selectedColumnIds]
  );

  if (!open) return null;

  const handleExport = () => {
    if (loading) {
      toast.warning('导出数据仍在读取中');
      return;
    }
    if (rows.length === 0) {
      toast.warning('没有可导出的数据');
      return;
    }
    if (selectedColumns.length === 0) {
      toast.warning('请选择至少一个导出字段');
      return;
    }
    exportRows(rows, selectedColumns, format, fileName || defaultName);
    toast.success('导出文件已生成');
    onClose();
  };

  const allSelected = selectedColumnIds.length === columns.length;
  const toggleAllColumns = () => {
    setSelectedColumnIds(allSelected ? [] : columns.map(column => columnId(column)));
  };

  const toggleColumn = (id: string) => {
    setSelectedColumnIds(current =>
      current.includes(id) ? current.filter(item => item !== id) : [...current, id]
    );
  };

  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-[1500] flex items-center justify-center px-4 py-6">
        <section className="kvm-dialog-panel flex max-h-[88vh] w-[min(94vw,680px)] flex-col overflow-hidden rounded-2xl shadow-2xl" role="dialog" aria-modal="true" aria-label={title}>
          <div className="flex items-start justify-between gap-4 border-b p-5" style={{ borderColor: 'var(--kvm-border)' }}>
            <div className="min-w-0">
              <h3 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>{title}</h3>
              <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{loading ? loadingText : `将当前筛选后的 ${rows.length} 条记录导出为文件`}</p>
            </div>
            <button type="button" onClick={onClose} className="kvm-action-button flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border" style={{ borderColor: 'var(--kvm-border)', color: 'var(--kvm-text-muted)', background: 'rgba(255,255,255,0.04)' }} aria-label="关闭"><XIcon size={15} /></button>
          </div>
          <div className="kvm-hidden-scrollbar flex-1 space-y-5 overflow-y-auto p-5">
            <div className="grid gap-4 md:grid-cols-[1fr_180px]">
              <label className="block space-y-1.5 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                <div>导出名称</div>
                <input value={fileName} onChange={event => setFileName(event.target.value)} placeholder={`${defaultName}-${localTimestamp()}`} className="h-10 w-full rounded-lg px-3 text-sm outline-none" style={{ background: 'var(--kvm-control-bg)', border: '1px solid var(--kvm-border)', color: 'var(--kvm-text)' }} />
              </label>
              <div className="space-y-1.5 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                <div>扩展名</div>
                <SelectMenu value={format} options={formatOptions} placeholder="选择格式" menuZIndex={1600} placement="bottom" onChange={value => setFormat(value as ExportFormat)} />
              </div>
            </div>
            <div className="space-y-3 rounded-xl border p-4" style={{ borderColor: 'var(--kvm-border)', background: 'rgba(255,255,255,0.022)' }}>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div className="text-xs font-semibold" style={{ color: 'var(--kvm-text)' }}>导出字段</div>
                  <div className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>默认导出全部字段，可按需取消</div>
                </div>
                <button type="button" onClick={toggleAllColumns} className="kvm-action-button rounded-lg border px-3 py-2 text-xs font-semibold" style={{ borderColor: 'var(--kvm-border)', color: 'var(--kvm-text-muted)', background: 'rgba(255,255,255,0.035)' }}>{allSelected ? '取消全选' : '全选'}</button>
              </div>
              <div className="grid grid-cols-2 gap-2 md:grid-cols-3">
                {columns.map(column => {
                  const id = columnId(column);
                  const selected = selectedColumnIds.includes(id);
                  return (
                    <button
                      key={id}
                      type="button"
                      onClick={() => toggleColumn(id)}
                      className="kvm-action-button flex h-10 min-w-0 items-center gap-2 rounded-lg border px-3 text-left text-sm transition"
                      style={{
                        borderColor: selected ? 'rgba(45,212,191,0.42)' : 'var(--kvm-border)',
                        background: selected ? 'rgba(45,212,191,0.1)' : 'var(--kvm-control-bg)',
                        color: selected ? 'var(--kvm-text)' : 'var(--kvm-text-muted)',
                      }}
                    >
                      <span className="flex h-4 w-4 shrink-0 items-center justify-center rounded border" style={{ borderColor: selected ? 'rgba(45,212,191,0.7)' : 'var(--kvm-border)', background: selected ? 'rgba(45,212,191,0.22)' : 'transparent' }}>{selected && <CheckIcon size={12} />}</span>
                      <span className="min-w-0 truncate">{column.header}</span>
                    </button>
                  );
                })}
              </div>
            </div>
            <div className="flex items-center gap-3 rounded-lg border p-3 text-sm" style={{ borderColor: 'var(--kvm-border)', color: 'var(--kvm-text-muted)', background: 'rgba(255,255,255,0.028)' }}>
              {loading ? <Loader2Icon size={16} className="animate-spin" /> : <FileDownIcon size={16} />}
              <span className="min-w-0 truncate">{previewName}</span>
              <span className="ml-auto shrink-0 text-xs">{loading ? '读取中' : `${selectedColumns.length}/${columns.length} 列`}</span>
            </div>
          </div>
          <div className="flex justify-end gap-2 border-t p-4" style={{ borderColor: 'var(--kvm-border)', background: 'rgba(255,255,255,0.018)' }}>
            <button type="button" onClick={onClose} className="kvm-action-button rounded-lg border px-3 py-2 text-sm" style={{ borderColor: 'var(--kvm-border)', color: 'var(--kvm-text-muted)', background: 'rgba(255,255,255,0.035)' }}>取消</button>
            <button type="button" disabled={loading} onClick={handleExport} className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-60" style={{ borderColor: 'rgba(59,130,246,0.38)', color: 'var(--kvm-accent-text)', background: 'rgba(59,130,246,0.1)' }}>{loading ? <Loader2Icon size={14} className="animate-spin" /> : <DownloadIcon size={14} />}{loading ? '读取中' : '导出'}</button>
          </div>
        </section>
      </div>
    </DialogPortal>
  );
}

function columnId<T>(column: ExportColumn<T>) {
  return column.id || column.header;
}
