import { FormEvent, useEffect, useState } from 'react'
import { ApiError } from '../api/client'
import {
  useCreateConnection,
  type CreateConnectionInput,
  type Tag,
  type TLSMode,
} from '../api/connections'
import Icon from './Icon'

interface Props {
  open: boolean
  onClose(): void
  onCreated?(id: number): void
}

const TAGS: Tag[] = ['local', 'dev', 'staging', 'production']
const TLS_MODES: TLSMode[] = ['disable', 'require', 'verify-ca', 'verify-full']

export default function ConnectionFormDialog({ open, onClose, onCreated }: Props) {
  const [input, setInput] = useState<CreateConnectionInput>(() => initial())
  const [error, setError] = useState<string | null>(null)
  const create = useCreateConnection()

  // Reset form when reopened.
  useEffect(() => {
    if (open) {
      setInput(initial())
      setError(null)
    }
  }, [open])

  if (!open) return null

  const needsPassphrase = input.tag === 'production'

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      const created = await create.mutateAsync(input)
      onCreated?.(created.id)
      onClose()
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 409) setError('A connection with this alias already exists')
        else setError(err.body.error || `Create failed (${err.status})`)
      } else {
        setError(err instanceof Error ? err.message : 'Create failed')
      }
    }
  }

  const update = <K extends keyof CreateConnectionInput>(k: K, v: CreateConnectionInput[K]) =>
    setInput((prev) => ({ ...prev, [k]: v }))

  return (
    <div
      role="dialog"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink-950/70 backdrop-blur-sm p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <form
        onSubmit={onSubmit}
        className="w-full max-w-lg bg-ink-800/80 backdrop-blur-md border border-ink-700 rounded-2xl shadow-card overflow-hidden"
      >
        <div className="px-5 py-4 border-b border-ink-700 flex items-center">
          <h2 className="text-ink-50 font-semibold text-[15px]">New connection</h2>
          <button
            type="button"
            onClick={onClose}
            className="ml-auto p-1.5 rounded-md text-ink-300 hover:text-ink-50 hover:bg-ink-700"
            aria-label="Close"
          >
            <Icon name="x" className="w-3.5 h-3.5" />
          </button>
        </div>

        <div className="px-5 py-4 space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <Field label="Alias">
              <Input value={input.alias} onChange={(v) => update('alias', v)} required placeholder="my-app-db" />
            </Field>
            <Field label="Tag">
              <Select
                value={input.tag}
                onChange={(v) => update('tag', v as Tag)}
                options={TAGS}
              />
            </Field>
          </div>

          <div className="grid grid-cols-[1fr_120px] gap-3">
            <Field label="Host">
              <Input value={input.host} onChange={(v) => update('host', v)} required placeholder="127.0.0.1" />
            </Field>
            <Field label="Port">
              <Input
                value={String(input.port)}
                onChange={(v) => update('port', Number(v) || 0)}
                required
                inputMode="numeric"
              />
            </Field>
          </div>

          <Field label="Database">
            <Input value={input.database} onChange={(v) => update('database', v)} required placeholder="postgres" />
          </Field>

          <div className="grid grid-cols-2 gap-3">
            <Field label="Username">
              <Input value={input.username} onChange={(v) => update('username', v)} required placeholder="postgres" />
            </Field>
            <Field label="Password">
              <Input type="password" value={input.password} onChange={(v) => update('password', v)} required />
            </Field>
          </div>

          <Field label="TLS mode">
            <Select
              value={input.tls_mode}
              onChange={(v) => update('tls_mode', v as TLSMode)}
              options={TLS_MODES}
            />
          </Field>

          <Field
            label={
              <span>
                Per-connection passphrase{' '}
                <span className="text-ink-500 font-normal">
                  ({needsPassphrase ? 'required for production' : 'optional, leave empty to skip'})
                </span>
              </span>
            }
          >
            <Input
              type="password"
              value={input.passphrase ?? ''}
              onChange={(v) => update('passphrase', v || undefined)}
              placeholder="rotate-this-secret"
              required={needsPassphrase}
            />
          </Field>

          {error && (
            <div className="p-2.5 rounded-md bg-accent-coral/10 border border-accent-coral/40 text-accent-coral text-[12px]">
              {error}
            </div>
          )}
        </div>

        <div className="px-5 py-3 border-t border-ink-700 flex justify-end gap-2 bg-ink-900/30">
          <button
            type="button"
            onClick={onClose}
            className="h-9 px-4 rounded-lg border border-ink-700 text-ink-200 hover:bg-ink-700 text-[13px] font-medium"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={create.isPending}
            className="h-9 px-4 rounded-lg bg-violet text-white font-medium text-[13px] hover:bg-violet-deep transition-colors shadow-glow disabled:opacity-50 disabled:shadow-none"
          >
            {create.isPending ? 'Saving…' : 'Create connection'}
          </button>
        </div>
      </form>
    </div>
  )
}

function initial(): CreateConnectionInput {
  return {
    alias: '',
    host: '127.0.0.1',
    port: 5432,
    tag: 'local',
    tls_mode: 'disable',
    username: 'postgres',
    password: '',
    database: 'postgres',
    passphrase: '',
  }
}

function Field({ label, children }: { label: React.ReactNode; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="text-ink-300 text-[11.5px] font-medium mb-1 block">{label}</span>
      {children}
    </label>
  )
}

function Input({
  value,
  onChange,
  type = 'text',
  required,
  placeholder,
  inputMode,
}: {
  value: string
  onChange(v: string): void
  type?: 'text' | 'password' | 'email'
  required?: boolean
  placeholder?: string
  inputMode?: 'numeric'
}) {
  return (
    <input
      value={value}
      onChange={(e) => onChange(e.target.value)}
      type={type}
      required={required}
      placeholder={placeholder}
      inputMode={inputMode}
      className="w-full h-9 px-3 rounded-lg bg-ink-900 border border-ink-700 focus:border-violet focus:outline-none text-[13px] text-ink-50 placeholder:text-ink-500"
    />
  )
}

function Select({
  value,
  onChange,
  options,
}: {
  value: string
  onChange(v: string): void
  options: readonly string[]
}) {
  return (
    <div className="relative">
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="appearance-none w-full h-9 pl-3 pr-9 rounded-lg bg-ink-900 border border-ink-700 hover:border-ink-600 focus:border-violet focus:outline-none text-[13px] text-ink-50"
      >
        {options.map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </select>
      <Icon name="chevron-down" className="w-3.5 h-3.5 absolute right-3 top-1/2 -translate-y-1/2 text-ink-400 pointer-events-none" />
    </div>
  )
}
