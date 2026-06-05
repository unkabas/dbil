import { useCallback, useMemo, useState } from 'react'
import type { CellValue } from '../api/schema'
import type { RowChange } from '../api/mutations'

// rowKeySep joins primary-key parts into a single stable key for a loaded row.
const ROW_KEY_SEP = ''

export type PK = Record<string, CellValue>

export function rowKeyFor(row: CellValue[], colIndex: Record<string, number>, pkCols: string[]): string {
  return pkCols.map((c) => String(row[colIndex[c]])).join(ROW_KEY_SEP)
}

export function pkFor(row: CellValue[], colIndex: Record<string, number>, pkCols: string[]): PK {
  const pk: PK = {}
  for (const c of pkCols) pk[c] = row[colIndex[c]]
  return pk
}

interface EditEntry {
  pk: PK
  set: Record<string, CellValue>
}

export interface DraftRow {
  id: number
  values: Record<string, CellValue>
}

export interface TableEditor {
  dirtyCount: number
  drafts: DraftRow[]
  // cellOverride reports a pending value for a loaded cell, if any.
  cellOverride(rowKey: string, col: string): { has: boolean; value: CellValue }
  isDeleted(rowKey: string): boolean
  editCell(rowKey: string, pk: PK, col: string, value: CellValue): void
  toggleDelete(rowKey: string, pk: PK): void
  addDraft(): void
  setDraftCell(id: number, col: string, value: CellValue): void
  removeDraft(id: number): void
  reset(): void
  buildChanges(): RowChange[]
}

// useTableEditor holds the DataGrip-style pending-change set: per-cell edits on
// loaded rows, whole-row deletes (keyed by primary key), and freshly drafted
// insert rows. Nothing touches the database until buildChanges() is submitted.
export function useTableEditor(): TableEditor {
  const [edits, setEdits] = useState<Record<string, EditEntry>>({})
  const [deletes, setDeletes] = useState<Record<string, PK>>({})
  const [drafts, setDrafts] = useState<DraftRow[]>([])
  const [nextDraftId, setNextDraftId] = useState(1)

  const editCell = useCallback((rowKey: string, pk: PK, col: string, value: CellValue) => {
    setEdits((prev) => {
      const entry = prev[rowKey] ?? { pk, set: {} }
      return { ...prev, [rowKey]: { pk: entry.pk, set: { ...entry.set, [col]: value } } }
    })
  }, [])

  const toggleDelete = useCallback((rowKey: string, pk: PK) => {
    setDeletes((prev) => {
      const next = { ...prev }
      if (next[rowKey]) delete next[rowKey]
      else next[rowKey] = pk
      return next
    })
  }, [])

  const addDraft = useCallback(() => {
    setDrafts((prev) => [...prev, { id: nextDraftId, values: {} }])
    setNextDraftId((n) => n + 1)
  }, [nextDraftId])

  const setDraftCell = useCallback((id: number, col: string, value: CellValue) => {
    setDrafts((prev) => prev.map((d) => (d.id === id ? { ...d, values: { ...d.values, [col]: value } } : d)))
  }, [])

  const removeDraft = useCallback((id: number) => {
    setDrafts((prev) => prev.filter((d) => d.id !== id))
  }, [])

  const reset = useCallback(() => {
    setEdits({})
    setDeletes({})
    setDrafts([])
  }, [])

  const cellOverride = useCallback(
    (rowKey: string, col: string) => {
      const entry = edits[rowKey]
      if (entry && col in entry.set) return { has: true, value: entry.set[col] }
      return { has: false, value: null as CellValue }
    },
    [edits],
  )

  const isDeleted = useCallback((rowKey: string) => rowKey in deletes, [deletes])

  const dirtyCount = useMemo(() => {
    const editedRows = Object.entries(edits).filter(
      ([key, e]) => !(key in deletes) && Object.keys(e.set).length > 0,
    ).length
    const draftRows = drafts.filter((d) => Object.keys(d.values).length > 0).length
    return editedRows + Object.keys(deletes).length + draftRows
  }, [edits, deletes, drafts])

  const buildChanges = useCallback((): RowChange[] => {
    const changes: RowChange[] = []
    for (const [key, e] of Object.entries(edits)) {
      if (key in deletes) continue
      if (Object.keys(e.set).length === 0) continue
      changes.push({ op: 'update', pk: e.pk, set: e.set })
    }
    for (const pk of Object.values(deletes)) {
      changes.push({ op: 'delete', pk })
    }
    for (const d of drafts) {
      if (Object.keys(d.values).length === 0) continue
      changes.push({ op: 'insert', values: d.values })
    }
    return changes
  }, [edits, deletes, drafts])

  return {
    dirtyCount,
    drafts,
    cellOverride,
    isDeleted,
    editCell,
    toggleDelete,
    addDraft,
    setDraftCell,
    removeDraft,
    reset,
    buildChanges,
  }
}
