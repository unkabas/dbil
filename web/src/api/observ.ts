import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from './client'

export interface OverviewSample {
  ts_ms: number
  tps: number
  cache_hit: number
  active_conns: number
  idle_conns: number
  rep_lag_ms?: number
}

export interface SlowRow {
  query_hash: string
  preview: string
  mean_ms: number
  p95_ms: number
  p99_ms: number
  calls: number
  total_ms: number
  rows_avg: number
}

export interface LockSession {
  pid: number
  user: string
  query: string
  age_ms: number
  state: string
  blocked_by: number[]
}

export interface LockChain {
  holder: LockSession
  blocked: LockSession[]
}

export interface OverviewResponse {
  samples: OverviewSample[]
}

export interface SlowResponse {
  taken_at_ms: number
  rows: SlowRow[]
}

export interface LocksResponse {
  chains: LockChain[]
}

const pollIntervalMs = (tag: string) => {
  switch (tag) {
    case 'production': return 5_000
    case 'staging':    return 10_000
    case 'dev':        return 30_000
    default:           return 60_000
  }
}

export function useOverview(connID: number | null, tag: string) {
  const sinceMs = Date.now() - 5 * 60 * 1000
  return useQuery({
    queryKey: ['observ', 'overview', connID],
    enabled: connID !== null && connID !== 0,
    queryFn: () =>
      apiFetch<OverviewResponse>(
        `/api/connections/${connID}/observ/overview?since=${sinceMs}`,
      ),
    refetchInterval: pollIntervalMs(tag),
    staleTime: 0,
  })
}

export function useSlowQueries(connID: number | null, tag: string) {
  return useQuery({
    queryKey: ['observ', 'slow', connID],
    enabled: connID !== null && connID !== 0,
    queryFn: () => apiFetch<SlowResponse>(`/api/connections/${connID}/observ/slow`),
    refetchInterval: pollIntervalMs(tag) * 2,
    staleTime: 0,
  })
}

export function useLocks(connID: number | null) {
  return useQuery({
    queryKey: ['observ', 'locks', connID],
    enabled: connID !== null && connID !== 0,
    queryFn: () => apiFetch<LocksResponse>(`/api/connections/${connID}/observ/locks`),
    refetchInterval: 8_000,
    staleTime: 0,
  })
}

export interface TerminateResponse {
  signalled: boolean
  pid: number
}

export function useTerminateBackend(connID: number | null) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (input: { pid: number; confirm?: boolean }) =>
      apiFetch<TerminateResponse>(`/api/connections/${connID}/locks/${input.pid}/terminate`, {
        method: 'POST',
        headers: input.confirm ? { 'X-Confirm': 'yes' } : undefined,
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['observ', 'locks', connID] })
    },
  })
}
