import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from './client'

export type Role = 'admin' | 'member' | 'viewer'

export interface User {
  id: number
  email: string
  role: Role
  must_rotate: boolean
  created_at: number
  updated_at: number
}

const KEY = ['users'] as const

export function useUsers() {
  return useQuery({
    queryKey: KEY,
    queryFn: () =>
      apiFetch<{ users: User[] | null }>('/api/users').then((r) => r.users ?? []),
  })
}

// CreateUserResult echoes the new user plus the one-time generated password.
export type CreateUserResult = User & { password: string }

export function useCreateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: { email: string; role: Role }) =>
      apiFetch<CreateUserResult>('/api/users', { body: input }),
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  })
}

export function useUpdateUserRole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, role }: { id: number; role: Role }) =>
      apiFetch<User>(`/api/users/${id}`, { method: 'PATCH', body: { role } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  })
}

export function useDeleteUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<void>(`/api/users/${id}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  })
}

export function useResetPassword() {
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<{ password: string }>(`/api/users/${id}/reset-password`, {
        method: 'POST',
      }),
  })
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (input: { current: string; new: string }) =>
      apiFetch<void>('/api/me/password', { method: 'POST', body: input }),
  })
}
