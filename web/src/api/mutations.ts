import { useMutation } from '@tanstack/react-query'
import { apiFetch } from './client'

export type RowChangeOp = 'update' | 'delete' | 'insert'

// A single typed edit. pk identifies the row for update/delete; set holds new
// values for update; values holds the columns for insert. null means SQL NULL.
export interface RowChange {
  op: RowChangeOp
  pk?: Record<string, unknown>
  set?: Record<string, unknown>
  values?: Record<string, unknown>
}

export interface MutateResult {
  rows_affected: number
  statements: number
}

export interface SubmitMutationsInput {
  id: number
  schema: string
  name: string
  changes: RowChange[]
  confirm?: boolean
  passphrase?: string
}

export function useSubmitMutations() {
  return useMutation({
    mutationFn: ({ id, schema, name, changes, confirm, passphrase }: SubmitMutationsInput) => {
      const headers: Record<string, string> = {}
      if (confirm) headers['X-Confirm'] = 'yes'
      if (passphrase) headers['X-Connection-Passphrase'] = passphrase
      const path = `/api/connections/${id}/table/${encodeURIComponent(
        schema,
      )}/${encodeURIComponent(name)}/mutations`
      return apiFetch<MutateResult>(path, { body: { changes }, headers })
    },
  })
}
