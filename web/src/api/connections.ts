import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from './client'

export type Tag = 'local' | 'dev' | 'staging' | 'production'
export type TLSMode = 'disable' | 'require' | 'verify-ca' | 'verify-full'

export interface Connection {
  id: number
  alias: string
  host: string
  port: number
  tag: Tag
  tls_mode: TLSMode
  requires_passphrase: boolean
  created_at: number
  updated_at: number
}

export interface CreateConnectionInput {
  alias: string
  host: string
  port: number
  tag: Tag
  tls_mode: TLSMode
  username: string
  password: string
  database: string
  passphrase?: string
}

export interface ProbeResponse {
  version: string
  superuser_ok: boolean
  has_pg_stat_statements: boolean
}

export interface QueryResult {
  columns: { name: string; type_name: string }[]
  rows: (string | number | boolean | null)[][]
  rows_affected: number
  command_tag: string
  duration_ms: number
  truncated: boolean
}

export interface QueryInput {
  id: number
  sql: string
  confirm?: boolean
  passphrase?: string
}

const KEY = ['connections'] as const

export function useConnections() {
  return useQuery({
    queryKey: KEY,
    queryFn: () => apiFetch<Connection[] | null>('/api/connections').then((v) => v ?? []),
  })
}

export function useCreateConnection() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateConnectionInput) =>
      apiFetch<Connection>('/api/connections', { body: input }),
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  })
}

export function useDeleteConnection() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<void>(`/api/connections/${id}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  })
}

export function useTestConnection() {
  return useMutation({
    mutationFn: ({ id, passphrase }: { id: number; passphrase?: string }) =>
      apiFetch<ProbeResponse>(`/api/connections/${id}/test`, {
        method: 'POST',
        headers: passphrase ? { 'X-Connection-Passphrase': passphrase } : undefined,
      }),
  })
}

export function useExecuteQuery() {
  return useMutation({
    mutationFn: ({ id, sql, confirm, passphrase }: QueryInput) => {
      const headers: Record<string, string> = {}
      if (confirm) headers['X-Confirm'] = 'yes'
      if (passphrase) headers['X-Connection-Passphrase'] = passphrase
      return apiFetch<QueryResult>(`/api/connections/${id}/query`, {
        body: { sql },
        headers,
      })
    },
  })
}
