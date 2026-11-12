/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { zipSync, strToU8 } from 'fflate'
import { parquetWriteBuffer } from 'hyparquet-writer'

/**
 * The formats a download may be written in.
 *
 * The file is built in the browser: the server returns the rows of the filter
 * and the panel writes the document the administrator asked for. That keeps the
 * gateway free of format libraries and it lets a single dialog offer formats
 * (Parquet, a SQL dump) that would otherwise be a dependency of the server.
 */
export type LogExportFormat =
  | 'csv'
  | 'json'
  | 'txt'
  | 'log'
  | 'sql'
  | 'parquet'

export type LogExportPackage = 'none' | 'zip'

/** One row of the export. The values are the columns of the usage logs. */
export type LogExportRow = Record<string, unknown>

export const LOG_EXPORT_FORMATS: LogExportFormat[] = [
  'csv',
  'json',
  'txt',
  'log',
  'sql',
  'parquet',
]

/** The extension of a format, used for the file name. */
export function exportExtension(format: LogExportFormat): string {
  switch (format) {
    case 'csv':
      return 'csv'
    case 'json':
      return 'json'
    case 'txt':
      return 'txt'
    case 'log':
      return 'log'
    case 'sql':
      return 'sql'
    case 'parquet':
      return 'parquet'
  }
}

/** The columns of the export, in the order of the documents. */
export function exportColumns(rows: LogExportRow[]): string[] {
  const columns: string[] = []
  const seen = new Set<string>()
  for (const row of rows) {
    for (const key of Object.keys(row)) {
      if (seen.has(key)) continue
      seen.add(key)
      columns.push(key)
    }
  }
  return columns
}

function csvValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  if (text === undefined) return ''
  if (/[",\n\r]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`
  }
  return text
}

function textValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function sqlValue(value: unknown): string {
  if (value === null || value === undefined) return 'NULL'
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  if (typeof value === 'boolean') return value ? '1' : '0'
  if (typeof value === 'object') return sqlValue(JSON.stringify(value))
  return `'${String(value).replace(/\\/g, '\\\\').replace(/'/g, "''")}'`
}

function sqlIdentifier(name: string): string {
  return `\`${name.replace(/`/g, '``')}\``
}

/** Writes the rows as a CSV document. */
export function toCsv(rows: LogExportRow[]): string {
  const columns = exportColumns(rows)
  const lines = [columns.map(csvValue).join(',')]
  for (const row of rows) {
    lines.push(columns.map((column) => csvValue(row[column])).join(','))
  }
  return lines.join('\n')
}

/** Writes the rows as a pretty printed JSON array. */
export function toJson(rows: LogExportRow[]): string {
  return JSON.stringify(rows, null, 2)
}

/** Writes the rows as an aligned text table. */
export function toTxt(rows: LogExportRow[]): string {
  const columns = exportColumns(rows)
  const widths = columns.map((column) =>
    Math.max(
      column.length,
      ...rows.map((row) => textValue(row[column]).length)
    )
  )
  const line = (cells: string[]) =>
    cells
      .map((cell, index) => cell.padEnd(widths[index] ?? cell.length))
      .join('  ')
      .trimEnd()
  const header = line(columns)
  const separator = line(widths.map((width) => '-'.repeat(width)))
  return [header, separator, ...rows.map((row) => line(columns.map((column) => textValue(row[column]))))].join('\n')
}

/**
 * Writes the rows as log lines: one line per request, the fields in a stable
 * order, which is what a `grep` and a log viewer understand.
 */
export function toLogLines(rows: LogExportRow[]): string {
  const columns = exportColumns(rows)
  return rows
    .map((row) =>
      columns
        .map((column) => `${column}=${textValue(row[column])}`)
        .join(' ')
    )
    .join('\n')
}

/** Writes the rows as INSERT statements of a portable SQL dump. */
export function toSql(rows: LogExportRow[], table = 'logs'): string {
  const columns = exportColumns(rows)
  const statements = [
    `-- usage logs export`,
    `-- table: ${table}`,
    `-- rows: ${rows.length}`,
  ]
  if (columns.length === 0) return statements.join('\n')
  const columnList = columns.map(sqlIdentifier).join(', ')
  for (const row of rows) {
    const values = columns.map((column) => sqlValue(row[column])).join(', ')
    statements.push(
      `INSERT INTO ${sqlIdentifier(table)} (${columnList}) VALUES (${values});`
    )
  }
  return statements.join('\n')
}

/**
 * Writes the rows as a Parquet file.
 *
 * Only the scalar columns are written: a nested value (the JSON of a request)
 * becomes its text, which is what a reader of the file wants to see and what
 * keeps the schema flat and readable by every tool.
 */
export function toParquet(rows: LogExportRow[]): Uint8Array {
  const columns = exportColumns(rows)
  const columnData = columns.map((column) => {
    const values = rows.map((row) => row[column])
    const allNumbers =
      values.length > 0 &&
      values.every(
        (value) =>
          value === null ||
          value === undefined ||
          (typeof value === 'number' && Number.isFinite(value))
      )
    if (allNumbers) {
      const numbers = values.map((value) =>
        typeof value === 'number' && Number.isFinite(value) ? value : null
      )
      return {
        name: column,
        data: numbers,
        type: 'DOUBLE' as const,
        nullable: true,
      }
    }
    const strings = values.map((value) => textValue(value))
    return {
      name: column,
      data: strings,
      type: 'STRING' as const,
      nullable: true,
    }
  })

  const buffer = parquetWriteBuffer({ columnData })
  return new Uint8Array(buffer)
}

/** Builds the bytes of one format. */
export function buildExportFile(
  rows: LogExportRow[],
  format: LogExportFormat
): Uint8Array {
  switch (format) {
    case 'csv':
      return strToU8(toCsv(rows))
    case 'json':
      return strToU8(toJson(rows))
    case 'txt':
      return strToU8(toTxt(rows))
    case 'log':
      return strToU8(toLogLines(rows))
    case 'sql':
      return strToU8(toSql(rows))
    case 'parquet':
      return toParquet(rows)
  }
}

/** The name of the file of a download. */
export function exportFileName(
  format: LogExportFormat,
  packageAs: LogExportPackage,
  now: Date = new Date()
): string {
  const stamp = now.toISOString().replace(/[:.]/g, '-').slice(0, 19)
  const base = `usage-logs-${stamp}`
  if (packageAs === 'zip') return `${base}.zip`
  return `${base}.${exportExtension(format)}`
}

/** Packs one file into a zip archive. */
export function buildExportArchive(
  rows: LogExportRow[],
  format: LogExportFormat,
  now: Date = new Date()
): Uint8Array {
  const stamp = now.toISOString().replace(/[:.]/g, '-').slice(0, 19)
  const name = `usage-logs-${stamp}.${exportExtension(format)}`
  const archive = zipSync({ [name]: buildExportFile(rows, format) })
  return archive
}

/** Hands a file to the browser as a download. */
export function downloadBytes(
  bytes: Uint8Array,
  fileName: string,
  mimeType = 'application/octet-stream'
): void {
  const blob = new Blob([bytes as unknown as BlobPart], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = fileName
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  // The URL is kept for a moment: a browser that starts the download slowly
  // would otherwise cancel it when the object is released at once.
  window.setTimeout(() => URL.revokeObjectURL(url), 10_000)
}
