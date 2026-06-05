import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from './client'

export type SSHAuthMethod = 'key' | 'password'

export interface SSHHost {
  id: number
  alias: string
  host: string
  port: number
  username: string
  auth_method: SSHAuthMethod
  host_key_fingerprint?: string
  requires_passphrase: boolean
  created_at: number
  updated_at: number
}

export interface CreateSSHHostInput {
  alias: string
  host: string
  port: number
  username: string
  auth_method: SSHAuthMethod
  secret: string // private key PEM or password
  key_passphrase?: string // optional, for an encrypted private key
  passphrase?: string // optional, wraps the secret at rest
}

export interface SSHTestResult {
  reachable: boolean
  host_key_fingerprint: string
}

const KEY = ['ssh-hosts'] as const

export function useSSHHosts() {
  return useQuery({
    queryKey: KEY,
    queryFn: () =>
      apiFetch<SSHHost[] | null>('/api/ssh-hosts').then((v) => v ?? []),
  })
}

export function useCreateSSHHost() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateSSHHostInput) =>
      apiFetch<SSHHost>('/api/ssh-hosts', { body: input }),
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  })
}

export function useDeleteSSHHost() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<void>(`/api/ssh-hosts/${id}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  })
}

export function useTestSSHHost() {
  return useMutation({
    mutationFn: ({ id, passphrase }: { id: number; passphrase?: string }) =>
      apiFetch<SSHTestResult>(`/api/ssh-hosts/${id}/test`, {
        method: 'POST',
        headers: passphrase ? { 'X-Connection-Passphrase': passphrase } : undefined,
      }),
  })
}
