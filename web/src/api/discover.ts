import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from './client'
import type { Tag } from './connections'

export type DiscoverSource = 'env' | 'docker'
export type DiscoverStatus = 'pending' | 'approved' | 'rejected' | 'unreachable'

export interface DiscoverEntry {
  id: number
  source: DiscoverSource
  source_key: string
  alias: string
  host: string
  port: number
  database: string
  username: string
  has_password: boolean
  tag: Tag
  status: DiscoverStatus
  last_seen_ms: number
  created_at_ms: number
  approved_conn_id?: number
}

export interface DiscoverResponse {
  entries: DiscoverEntry[]
}

export interface ApproveResponse {
  connection_id: number
}

export function useDiscovered() {
  return useQuery({
    queryKey: ['discover'],
    queryFn: () => apiFetch<DiscoverResponse>('/api/discover'),
    refetchInterval: 30_000,
    staleTime: 0,
  })
}

export function useApproveDiscovered() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (input: { id: number; passphrase?: string }) =>
      apiFetch<ApproveResponse>(`/api/discover/${input.id}/approve`, {
        method: 'POST',
        body: { passphrase: input.passphrase ?? '' },
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['discover'] })
      void qc.invalidateQueries({ queryKey: ['connections'] })
    },
  })
}

export function useRejectDiscovered() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: number) =>
      apiFetch<void>(`/api/discover/${id}/reject`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['discover'] })
    },
  })
}
