import { useQuery } from '@tanstack/react-query'
import { apiFetch, getStoredToken } from './client'

export interface FKRef {
  table: string
  column: string
}

export interface SchemaColumn {
  name: string
  type: string
  nullable: boolean
  pk: boolean
  unique: boolean
  fk?: FKRef
}

export interface SchemaTable {
  schema: string
  name: string
  rows: number
  size_bytes: number
  columns: SchemaColumn[]
}

export interface SchemaNamespace {
  name: string
  tables: SchemaTable[]
}

export interface SchemaDoc {
  schemas: SchemaNamespace[]
}

export interface RowsColumn {
  name: string
  type_name: string
}

export type CellValue = string | number | boolean | null

export interface RowsResponse {
  columns: RowsColumn[]
  rows: CellValue[][]
  estimated_total: number
}

export interface TableFilter {
  column: string
  values: CellValue[]
}

export interface SearchRowsResponse {
  columns: RowsColumn[]
  rows: CellValue[][]
  filtered_total: number
  truncated: boolean
}

export interface DistinctValue {
  value: CellValue
  count: number
}

export interface DistinctValuesResponse {
  values: DistinctValue[]
  truncated: boolean
}

export function useSchema(connID: number | null) {
  return useQuery({
    queryKey: ['schema', connID],
    enabled: connID !== null && connID !== 0,
    queryFn: () => apiFetch<SchemaDoc>(`/api/connections/${connID}/schema`),
    staleTime: 60_000,
  })
}

export function useTableRows(
  connID: number | null,
  schema: string | null,
  name: string | null,
  page: number,
  pageSize = 50,
  filters: TableFilter[] = [],
) {
  const hasFilters = filters.some((f) => f.values.length > 0)
  return useQuery({
    queryKey: ['table-rows', connID, schema, name, page, pageSize, filters],
    enabled: connID !== null && connID !== 0 && !!schema && !!name,
    queryFn: async () => {
      const base = `/api/connections/${connID}/table/${encodeURIComponent(schema!)}/${encodeURIComponent(name!)}`
      if (hasFilters) {
        const resp = await apiFetch<SearchRowsResponse>(`${base}/rows/search`, {
          method: 'POST',
          body: { page, page_size: pageSize, filters },
        })
        return { columns: resp.columns, rows: resp.rows, estimated_total: resp.filtered_total } satisfies RowsResponse
      }
      return apiFetch<RowsResponse>(`${base}/rows?page=${page}&page_size=${pageSize}`)
    },
    staleTime: 0,
  })
}

export function fetchDistinctValues(
  connID: number,
  schema: string,
  name: string,
  column: string,
  filters: TableFilter[],
) {
  return apiFetch<DistinctValuesResponse>(
    `/api/connections/${connID}/table/${encodeURIComponent(schema)}/${encodeURIComponent(
      name,
    )}/columns/${encodeURIComponent(column)}/values`,
    { method: 'POST', body: { filters } },
  )
}

export async function exportTable(
  connID: number,
  schema: string,
  name: string,
  format: 'csv' | 'json' | 'xlsx',
  scope: 'filtered' | 'all',
  filters: TableFilter[],
) {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const token = getStoredToken()
  if (token) headers.Authorization = `Bearer ${token}`
  const r = await fetch(
    `/api/connections/${connID}/table/${encodeURIComponent(schema)}/${encodeURIComponent(name)}/export`,
    {
      method: 'POST',
      headers,
      body: JSON.stringify({ format, scope, filters }),
    },
  )
  if (!r.ok) {
    const text = await r.text()
    throw new Error(text || `export failed: HTTP ${r.status}`)
  }
  const blob = await r.blob()
  const filename = filenameFromDisposition(r.headers.get('Content-Disposition')) ?? `${schema}.${name}.${format}`
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
  return {
    truncated: r.headers.get('X-DBIL-Export-Truncated') === 'true',
    limit: Number(r.headers.get('X-DBIL-Export-Limit') || 0),
  }
}

function filenameFromDisposition(v: string | null): string | null {
  if (!v) return null
  const m = /filename="([^"]+)"/.exec(v)
  return m?.[1] ?? null
}
