export type ExportFormat = 'csv' | 'txt' | 'xlsx' | 'xls';

export type ExportColumn<T> = {
  id?: string;
  header: string;
  value: (row: T) => unknown;
};

type ExportCellKind = 'text' | 'number' | 'percent' | 'gb';

type ExportCell = {
  text: string;
  kind: ExportCellKind;
  numeric?: number;
};

type CellMatrix = ExportCell[][];

export function exportRows<T>(
  rows: T[],
  columns: ExportColumn<T>[],
  format: ExportFormat,
  fileName: string
) {
  const matrix = buildMatrix(rows, columns);
  const safeName = sanitizeExportFileName(fileName || `export-${localTimestamp()}`);
  const output = buildExportBlob(matrix, format);
  downloadBlob(output.blob, `${safeName}.${format}`);
}

export function localTimestamp() {
  const date = new Date();
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}${pad(date.getMonth() + 1)}${pad(date.getDate())}${pad(date.getHours())}${pad(date.getMinutes())}`;
}

export function sanitizeExportFileName(value: string) {
  const cleaned = value
    .trim()
    .replace(/[<>:"/\\|?*\u0000-\u001f]+/g, '-')
    .replace(/\s+/g, '-');
  return cleaned || `export-${localTimestamp()}`;
}

function buildMatrix<T>(rows: T[], columns: ExportColumn<T>[]): CellMatrix {
  return [
    columns.map(column => ({ text: column.header, kind: 'text' as const })),
    ...rows.map(row => columns.map(column => toExportCell(column.value(row)))),
  ];
}

function buildExportBlob(matrix: CellMatrix, format: ExportFormat) {
  if (format === 'csv') {
    return { blob: new Blob([toDelimited(matrix, ',')], { type: 'text/csv;charset=utf-8' }) };
  }
  if (format === 'txt') {
    return { blob: new Blob([toFixedWidthTable(matrix)], { type: 'text/plain;charset=utf-8' }) };
  }
  if (format === 'xls') {
    const widths = columnWidths(matrix);
    const html = `<!doctype html><html xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:x="urn:schemas-microsoft-com:office:excel"><head><meta charset="utf-8"><style>
      table { border-collapse: collapse; }
      col { mso-width-source: userset; }
      td, th { border: 1px solid #d9e2ec; padding: 4px 8px; text-align: center; vertical-align: middle; background-color: transparent; font-family: "宋体", SimSun, serif; white-space: nowrap; }
      th { font-weight: 700; mso-number-format: "\\@"; }
      td.text { mso-number-format: "\\@"; }
      td.number { mso-number-format: "0"; }
      td.percent { mso-number-format: "0%"; }
      td.gb { mso-number-format: "0\\ \\&quot;GB\\&quot;"; }
    </style></head><body><table><colgroup>${widths
      .map(width => `<col style="width:${Math.round(width * 11 + 34)}px">`)
      .join('')}</colgroup>${matrix
      .map(
        (row, rowIndex) =>
          `<tr>${row.map(cell => renderHtmlCell(cell, rowIndex === 0)).join('')}</tr>`
      )
      .join('')}</table></body></html>`;
    return { blob: new Blob([html], { type: 'application/vnd.ms-excel;charset=utf-8' }) };
  }
  return {
    blob: new Blob([buildXlsx(matrix)], {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    }),
  };
}

function toDelimited(matrix: CellMatrix, delimiter: string) {
  return (
    '\ufeff' +
    matrix
      .map(row => row.map(cell => quoteDelimited(cell.text, delimiter)).join(delimiter))
      .join('\r\n')
  );
}

function toFixedWidthTable(matrix: CellMatrix) {
  if (matrix.length === 0) return '\ufeff';

  const widths = columnWidths(matrix).map(width => Math.ceil((width + 4) / 8) * 8);
  const starts = columnStarts(widths);
  const output = matrix.map(row => alignTextRow(row, starts));
  return '\ufeff' + output.join('\r\n');
}

function quoteDelimited(value: string, delimiter: string) {
  if (!value.includes(delimiter) && !/["\r\n]/.test(value)) return value;
  return `"${value.replace(/"/g, '""')}"`;
}

function toExportCell(value: unknown): ExportCell {
  const text = stringifyCell(value);
  const normalized = normalizeTextCell(text);

  if (typeof value === 'number' && Number.isFinite(value)) {
    return { text, kind: 'number', numeric: value };
  }

  const percentMatch = normalized.match(/^(-?\d+(?:\.\d+)?)%$/);
  if (percentMatch) {
    return { text: normalized, kind: 'percent', numeric: Number(percentMatch[1]) / 100 };
  }

  const gbMatch = normalized.match(/^(-?\d+(?:\.\d+)?)\s*GB$/i);
  if (gbMatch) {
    return { text: normalized.replace(/\s*GB$/i, ' GB'), kind: 'gb', numeric: Number(gbMatch[1]) };
  }

  const numberMatch = normalized.match(/^-?\d+(?:\.\d+)?$/);
  if (numberMatch) {
    return { text: normalized, kind: 'number', numeric: Number(normalized) };
  }

  return { text, kind: 'text' };
}

function stringifyCell(value: unknown): string {
  if (value === null || value === undefined) return '';
  if (value instanceof Date) return Number.isNaN(value.getTime()) ? '' : value.toLocaleString();
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

function normalizeTextCell(value: string) {
  return value.replace(/\s+/g, ' ').trim();
}

function columnWidths(matrix: CellMatrix) {
  const columnCount = matrix.reduce((max, row) => Math.max(max, row.length), 0);
  return Array.from({ length: columnCount }, (_, columnIndex) => {
    const width = matrix.reduce(
      (max, row) => Math.max(max, displayWidth(normalizeTextCell(row[columnIndex]?.text ?? ''))),
      0
    );
    return Math.min(Math.max(width + 2, 6), 72);
  });
}

function padCell(value: string, width: number, kind: ExportCellKind | 'center') {
  const padding = Math.max(0, width - displayWidth(value));
  if (kind === 'number' || kind === 'percent' || kind === 'gb') {
    return `${' '.repeat(padding)}${value}`;
  }
  if (kind !== 'center') {
    return `${value}${' '.repeat(padding)}`;
  }
  const left = Math.floor(padding / 2);
  const right = padding - left;
  return `${' '.repeat(left)}${value}${' '.repeat(right)}`;
}

function columnStarts(widths: number[]) {
  const starts: number[] = [];
  let next = 0;
  for (const width of widths) {
    starts.push(next);
    next += width;
  }
  return starts;
}

function alignTextRow(row: ExportCell[], starts: number[]) {
  let output = '';
  let position = 0;
  for (let index = 0; index < row.length; index += 1) {
    const value = normalizeTextCell(row[index]?.text ?? '');
    const target = starts[index] ?? position;
    if (position < target) {
      output += ' '.repeat(target - position);
      position = target;
    }
    output += value;
    position += displayWidth(value);
  }
  return output.trimEnd();
}

function displayWidth(value: string) {
  return Array.from(value).reduce((width, char) => width + (isWideChar(char) ? 2 : 1), 0);
}

function isWideChar(char: string) {
  const code = char.codePointAt(0) ?? 0;
  return (
    (code >= 0x1100 && code <= 0x115f) ||
    (code >= 0x2e80 && code <= 0xa4cf) ||
    (code >= 0xac00 && code <= 0xd7a3) ||
    (code >= 0xf900 && code <= 0xfaff) ||
    (code >= 0xfe10 && code <= 0xfe19) ||
    (code >= 0xfe30 && code <= 0xfe6f) ||
    (code >= 0xff00 && code <= 0xff60) ||
    (code >= 0xffe0 && code <= 0xffe6)
  );
}

function escapeXml(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function escapeHtml(value: string) {
  return escapeXml(value).replace(/'/g, '&#39;');
}

function escapeHtmlAttribute(value: string) {
  return escapeHtml(value).replace(/"/g, '&quot;');
}

function renderHtmlCell(cell: ExportCell, isHeader: boolean) {
  if (isHeader || cell.numeric === undefined || cell.kind === 'text') {
    return `<${isHeader ? 'th' : 'td'} class="text">${escapeHtml(cell.text)}</${isHeader ? 'th' : 'td'}>`;
  }

  return `<td class="${cell.kind}" title="${escapeHtmlAttribute(cell.text)}">${cell.numeric}</td>`;
}

function downloadBlob(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = fileName;
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function buildXlsx(matrix: CellMatrix) {
  const files = new Map<string, Uint8Array>();
  files.set(
    '[Content_Types].xml',
    encodeText(
      `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`
    )
  );
  files.set(
    '_rels/.rels',
    encodeText(
      `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`
    )
  );
  files.set(
    'xl/workbook.xml',
    encodeText(
      `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="导出数据" sheetId="1" r:id="rId1"/></sheets></workbook>`
    )
  );
  files.set(
    'xl/_rels/workbook.xml.rels',
    encodeText(
      `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`
    )
  );
  files.set('xl/styles.xml', encodeText(buildStylesXml()));
  files.set('xl/worksheets/sheet1.xml', encodeText(buildSheetXml(matrix)));
  return zipFiles(files);
}

function buildSheetXml(matrix: CellMatrix) {
  const cols = columnWidths(matrix)
    .map(
      (width, index) =>
        `<col min="${index + 1}" max="${index + 1}" width="${Math.min(Math.max(width + 2, 9), 80)}" customWidth="1"/>`
    )
    .join('');
  const rows = matrix
    .map((row, rowIndex) => {
      const cells = row
        .map((cell, columnIndex) => renderXlsxCell(cell, rowIndex, columnIndex))
        .join('');
      return `<row r="${rowIndex + 1}">${cells}</row>`;
    })
    .join('');
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><cols>${cols}</cols><sheetData>${rows}</sheetData></worksheet>`;
}

function buildStylesXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><numFmts count="2"><numFmt numFmtId="164" formatCode="0%"/><numFmt numFmtId="165" formatCode="0 &quot;GB&quot;"/></numFmts><fonts count="2"><font><sz val="11"/><name val="宋体"/><family val="3"/><charset val="134"/></font><font><b/><sz val="11"/><name val="宋体"/><family val="3"/><charset val="134"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border><border><left style="thin"><color rgb="FFD9E2EC"/></left><right style="thin"><color rgb="FFD9E2EC"/></right><top style="thin"><color rgb="FFD9E2EC"/></top><bottom style="thin"><color rgb="FFD9E2EC"/></bottom><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="6"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="49" fontId="0" fillId="0" borderId="1" xfId="0" applyAlignment="1" applyBorder="1" applyNumberFormat="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="49" fontId="1" fillId="0" borderId="1" xfId="0" applyAlignment="1" applyBorder="1" applyFont="1" applyNumberFormat="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyAlignment="1" applyBorder="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="164" fontId="0" fillId="0" borderId="1" xfId="0" applyAlignment="1" applyBorder="1" applyNumberFormat="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="165" fontId="0" fillId="0" borderId="1" xfId="0" applyAlignment="1" applyBorder="1" applyNumberFormat="1"><alignment horizontal="center" vertical="center"/></xf></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`;
}

function renderXlsxCell(cell: ExportCell, rowIndex: number, columnIndex: number) {
  const ref = cellRef(rowIndex, columnIndex);
  if (rowIndex === 0 || cell.numeric === undefined || cell.kind === 'text') {
    return `<c r="${ref}" s="${rowIndex === 0 ? 2 : 1}" t="inlineStr"><is><t>${escapeXml(cell.text)}</t></is></c>`;
  }
  return `<c r="${ref}" s="${cellStyleIndex(cell.kind)}"><v>${cell.numeric}</v></c>`;
}

function cellStyleIndex(kind: ExportCellKind) {
  if (kind === 'percent') return 4;
  if (kind === 'gb') return 5;
  if (kind === 'number') return 3;
  return 1;
}

function cellRef(rowIndex: number, columnIndex: number) {
  let column = '';
  let value = columnIndex + 1;
  while (value > 0) {
    const remainder = (value - 1) % 26;
    column = String.fromCharCode(65 + remainder) + column;
    value = Math.floor((value - 1) / 26);
  }
  return `${column}${rowIndex + 1}`;
}

function encodeText(value: string) {
  return new TextEncoder().encode(value);
}

function zipFiles(files: Map<string, Uint8Array>) {
  const chunks: Uint8Array[] = [];
  const central: Uint8Array[] = [];
  let offset = 0;
  for (const [name, data] of files) {
    const nameBytes = encodeText(name);
    const crc = crc32(data);
    const local = concatBytes(
      u32(0x04034b50),
      u16(20),
      u16(0),
      u16(0),
      u16(0),
      u16(0),
      u32(crc),
      u32(data.length),
      u32(data.length),
      u16(nameBytes.length),
      u16(0),
      nameBytes,
      data
    );
    chunks.push(local);
    central.push(
      concatBytes(
        u32(0x02014b50),
        u16(20),
        u16(20),
        u16(0),
        u16(0),
        u16(0),
        u16(0),
        u32(crc),
        u32(data.length),
        u32(data.length),
        u16(nameBytes.length),
        u16(0),
        u16(0),
        u16(0),
        u16(0),
        u32(0),
        u32(offset),
        nameBytes
      )
    );
    offset += local.length;
  }
  const centralSize = central.reduce((sum, item) => sum + item.length, 0);
  return concatBytes(
    ...chunks,
    ...central,
    u32(0x06054b50),
    u16(0),
    u16(0),
    u16(files.size),
    u16(files.size),
    u32(centralSize),
    u32(offset),
    u16(0)
  );
}

function concatBytes(...parts: Uint8Array[]) {
  const length = parts.reduce((sum, part) => sum + part.length, 0);
  const output = new Uint8Array(length);
  let offset = 0;
  for (const part of parts) {
    output.set(part, offset);
    offset += part.length;
  }
  return output;
}

function u16(value: number) {
  const bytes = new Uint8Array(2);
  new DataView(bytes.buffer).setUint16(0, value, true);
  return bytes;
}

function u32(value: number) {
  const bytes = new Uint8Array(4);
  new DataView(bytes.buffer).setUint32(0, value >>> 0, true);
  return bytes;
}

const crcTable = new Uint32Array(256).map((_, index) => {
  let crc = index;
  for (let bit = 0; bit < 8; bit += 1) {
    crc = crc & 1 ? 0xedb88320 ^ (crc >>> 1) : crc >>> 1;
  }
  return crc >>> 0;
});

function crc32(data: Uint8Array) {
  let crc = 0xffffffff;
  for (const byte of data) {
    crc = crcTable[(crc ^ byte) & 0xff] ^ (crc >>> 8);
  }
  return (crc ^ 0xffffffff) >>> 0;
}
