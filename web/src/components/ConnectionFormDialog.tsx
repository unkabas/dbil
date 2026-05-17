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
      className="scrim"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
      style={{ display: 'grid', placeItems: 'center', padding: 24 }}
    >
      <form
        onSubmit={onSubmit}
        style={{
          width: '100%',
          maxWidth: 520,
          background: 'var(--bg-2)',
          border: '1px solid var(--line-2)',
          borderRadius: 14,
          boxShadow: 'var(--shadow-pop)',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            padding: '14px 18px',
            borderBottom: '1px solid var(--line-1)',
            display: 'flex',
            alignItems: 'center',
          }}
        >
          <h2 style={{ fontSize: 14, fontWeight: 600, letterSpacing: '-0.01em', color: 'var(--fg-1)', margin: 0 }}>
            New connection
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="link-btn"
            aria-label="Close"
            style={{ marginLeft: 'auto', padding: 4 }}
          >
            <Icon name="x" size={14} />
          </button>
        </div>

        <div style={{ padding: 18, display: 'grid', gap: 12 }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 140px', gap: 12 }}>
            <Field label="Alias">
              <Input value={input.alias} onChange={(v) => update('alias', v)} required placeholder="my-app-db" />
            </Field>
            <Field label="Tag">
              <Select value={input.tag} onChange={(v) => update('tag', v as Tag)} options={TAGS} />
            </Field>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 120px', gap: 12 }}>
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

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Field label="Username">
              <Input
                value={input.username}
                onChange={(v) => update('username', v)}
                required
                placeholder="postgres"
              />
            </Field>
            <Field label="Password">
              <Input
                type="password"
                value={input.password}
                onChange={(v) => update('password', v)}
                required
              />
            </Field>
          </div>

          <Field label="TLS mode">
            <Select value={input.tls_mode} onChange={(v) => update('tls_mode', v as TLSMode)} options={TLS_MODES} />
          </Field>

          <Field
            label={
              <>
                Per-connection passphrase{' '}
                <span style={{ color: 'var(--fg-4)', fontWeight: 400 }}>
                  ({needsPassphrase ? 'required for production' : 'optional'})
                </span>
              </>
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
            <div
              style={{
                padding: 10,
                borderRadius: 7,
                background: 'var(--danger-soft)',
                border: '1px solid rgba(255,107,122,0.3)',
                color: 'var(--danger)',
                fontSize: 12,
              }}
            >
              {error}
            </div>
          )}
        </div>

        <div
          style={{
            padding: '12px 18px',
            borderTop: '1px solid var(--line-1)',
            display: 'flex',
            justifyContent: 'flex-end',
            gap: 8,
            background: 'var(--bg-1)',
          }}
        >
          <button type="button" onClick={onClose} className="btn-gh" style={{ height: 32 }}>
            Cancel
          </button>
          <button type="submit" className="btn-pri" disabled={create.isPending} style={{ height: 32 }}>
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
    <label style={{ display: 'block' }}>
      <span
        style={{
          fontSize: 11,
          color: 'var(--fg-3)',
          fontWeight: 500,
          marginBottom: 5,
          display: 'block',
          letterSpacing: '0.02em',
        }}
      >
        {label}
      </span>
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
  type?: 'text' | 'password'
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
      style={{
        width: '100%',
        height: 32,
        padding: '0 10px',
        borderRadius: 7,
        background: 'var(--bg-1)',
        border: '1px solid var(--line-2)',
        color: 'var(--fg-1)',
        fontSize: 12.5,
        outline: 0,
        fontFamily: 'inherit',
      }}
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
    <div style={{ position: 'relative' }}>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        style={{
          appearance: 'none',
          width: '100%',
          height: 32,
          padding: '0 28px 0 10px',
          borderRadius: 7,
          background: 'var(--bg-1)',
          border: '1px solid var(--line-2)',
          color: 'var(--fg-1)',
          fontSize: 12.5,
          outline: 0,
          fontFamily: 'inherit',
        }}
      >
        {options.map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </select>
      <Icon
        name="chev"
        size={12}
        style={{
          position: 'absolute',
          right: 10,
          top: '50%',
          transform: 'translateY(-50%)',
          color: 'var(--fg-4)',
          pointerEvents: 'none',
        }}
      />
    </div>
  )
}
