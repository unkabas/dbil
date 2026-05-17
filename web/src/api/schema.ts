import { useQuery } from '@tanstack/react-query'
import { apiFetch } from './client'

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
) {
  return useQuery({
    queryKey: ['table-rows', connID, schema, name, page, pageSize],
    enabled: connID !== null && connID !== 0 && !!schema && !!name,
    queryFn: () =>
      apiFetch<RowsResponse>(
        `/api/connections/${connID}/table/${encodeURIComponent(schema!)}/${encodeURIComponent(
          name!,
        )}/rows?page=${page}&page_size=${pageSize}`,
      ),
    staleTime: 0,
  })
}
