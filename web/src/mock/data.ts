// Mock data driving the schema viz + query page until the API client is wired.

export type Tag = 'local' | 'dev' | 'staging' | 'production'

export interface MockConnection {
  id: number
  alias: string
  host: string
  port: number
  database: string
  tag: Tag
  tls_mode: 'disable' | 'require' | 'verify-ca' | 'verify-full'
}

export interface MockColumn {
  name: string
  type: string
  nullable?: boolean
  pk?: boolean
  fk?: { table: string; column: string }
  unique?: boolean
}

export interface MockTable {
  schema: string
  name: string
  rows: number
  columns: MockColumn[]
  description?: string
  size?: string
  pos?: { x: number; y: number }
}

export interface MockResult {
  columns: { name: string; typeName: string }[]
  rows: (string | number | boolean | null)[][]
  rowsAffected: number
  commandTag: string
  durationMs: number
  truncated: boolean
}

export const mockConnections: MockConnection[] = [
  {
    id: 1,
    alias: 'local-postgres',
    host: '127.0.0.1',
    port: 5432,
    database: 'appdb',
    tag: 'local',
    tls_mode: 'disable',
  },
  {
    id: 2,
    alias: 'staging-readonly',
    host: 'staging.db.internal',
    port: 5432,
    database: 'app_staging',
    tag: 'staging',
    tls_mode: 'require',
  },
  {
    id: 3,
    alias: 'prod-readonly',
    host: 'prod.db.internal',
    port: 5432,
    database: 'app_prod',
    tag: 'production',
    tls_mode: 'verify-full',
  },
]

const tables: Record<number, MockTable[]> = {
  1: [
    {
      schema: 'public', name: 'users', rows: 1_204_551, size: '412 MB',
      description: 'Application user accounts',
      pos: { x: 60, y: 60 },
      columns: [
        { name: 'id',            type: 'int4',        pk: true },
        { name: 'email',         type: 'text',        unique: true },
        { name: 'name',          type: 'text',        nullable: true },
        { name: 'tier',          type: 'user_tier' },
        { name: 'created_at',    type: 'timestamptz' },
        { name: 'last_login_at', type: 'timestamptz', nullable: true },
      ],
    },
    {
      schema: 'public', name: 'orders', rows: 9_482_103, size: '2.4 GB',
      description: 'Customer orders',
      pos: { x: 480, y: 60 },
      columns: [
        { name: 'id',          type: 'int8',         pk: true },
        { name: 'user_id',     type: 'int4',         fk: { table: 'users', column: 'id' } },
        { name: 'total_cents', type: 'int4' },
        { name: 'currency',    type: 'char(3)' },
        { name: 'status',      type: 'order_status' },
        { name: 'placed_at',   type: 'timestamptz' },
      ],
    },
    {
      schema: 'public', name: 'sessions', rows: 47_812, size: '32 MB',
      pos: { x: 900, y: 60 },
      columns: [
        { name: 'id',         type: 'text',        pk: true },
        { name: 'user_id',    type: 'int4',        fk: { table: 'users', column: 'id' } },
        { name: 'created_at', type: 'timestamptz' },
        { name: 'expires_at', type: 'timestamptz' },
        { name: 'ip',         type: 'inet',        nullable: true },
      ],
    },
    {
      schema: 'public', name: 'products', rows: 1240, size: '0.8 MB',
      pos: { x: 60, y: 320 },
      columns: [
        { name: 'id',          type: 'int4', pk: true },
        { name: 'sku',         type: 'text', unique: true },
        { name: 'name',        type: 'text' },
        { name: 'price_cents', type: 'int4' },
        { name: 'inventory',   type: 'int4' },
      ],
    },
    {
      schema: 'public', name: 'order_items', rows: 28_491_002, size: '8.1 GB',
      pos: { x: 480, y: 320 },
      columns: [
        { name: 'order_id',    type: 'int8', fk: { table: 'orders',   column: 'id' } },
        { name: 'product_id',  type: 'int4', fk: { table: 'products', column: 'id' } },
        { name: 'quantity',    type: 'int4' },
        { name: 'price_cents', type: 'int4' },
      ],
    },
    {
      schema: 'audit', name: 'audit_log', rows: 1_103_482, size: '612 MB',
      description: 'Tamper-evident audit chain',
      pos: { x: 900, y: 320 },
      columns: [
        { name: 'id',        type: 'int8',  pk: true },
        { name: 'ts_ns',     type: 'int8' },
        { name: 'user_id',   type: 'text' },
        { name: 'action',    type: 'text' },
        { name: 'resource',  type: 'text' },
        { name: 'prev_hash', type: 'bytea' },
      ],
    },
  ],
  2: [
    {
      schema: 'public',
      name: 'users',
      rows: 8042,
      columns: [
        { name: 'id', type: 'int4', pk: true },
        { name: 'email', type: 'text' },
        { name: 'created_at', type: 'timestamptz' },
      ],
    },
    {
      schema: 'public',
      name: 'orders',
      rows: 42105,
      columns: [
        { name: 'id', type: 'int8', pk: true },
        { name: 'user_id', type: 'int4', fk: { table: 'users', column: 'id' } },
        { name: 'total_cents', type: 'int4' },
        { name: 'placed_at', type: 'timestamptz' },
      ],
    },
  ],
  3: [
    {
      schema: 'public',
      name: 'users',
      rows: 1_204_551,
      columns: [
        { name: 'id', type: 'int4', pk: true },
        { name: 'email', type: 'text' },
        { name: 'name', type: 'text', nullable: true },
        { name: 'created_at', type: 'timestamptz' },
      ],
    },
    {
      schema: 'public',
      name: 'orders',
      rows: 9_482_103,
      columns: [
        { name: 'id', type: 'int8', pk: true },
        { name: 'user_id', type: 'int4', fk: { table: 'users', column: 'id' } },
        { name: 'total_cents', type: 'int4' },
        { name: 'placed_at', type: 'timestamptz' },
      ],
    },
    {
      schema: 'public',
      name: 'products',
      rows: 1240,
      columns: [
        { name: 'id', type: 'int4', pk: true },
        { name: 'sku', type: 'text' },
        { name: 'name', type: 'text' },
        { name: 'price_cents', type: 'int4' },
      ],
    },
    {
      schema: 'public',
      name: 'order_items',
      rows: 28_491_002,
      columns: [
        { name: 'order_id', type: 'int8', fk: { table: 'orders', column: 'id' } },
        { name: 'product_id', type: 'int4', fk: { table: 'products', column: 'id' } },
        { name: 'quantity', type: 'int4' },
      ],
    },
  ],
}

export function tablesFor(connID: number): MockTable[] {
  // Real connection IDs from the backend won't collide with our 1/2/3 keys,
  // so fall back to the canonical sample schema for any unknown id. Plan 7
  // replaces this with real pg_catalog-driven introspection.
  return tables[connID] ?? tables[1] ?? []
}

export function findTable(connID: number, schema: string, name: string): MockTable | undefined {
  return tablesFor(connID).find((t) => t.schema === schema && t.name === name)
}

// Mock rows generator — produces deterministic sample data for any table by
// hashing column names + types. Good enough to demo the data viewer without
// hand-curating every table.
export function mockRowsFor(table: MockTable, count = 24): (string | number | boolean | null)[][] {
  const rows: (string | number | boolean | null)[][] = []
  for (let i = 0; i < count; i++) {
    rows.push(
      table.columns.map((c) => {
        if (c.nullable && i % 7 === 3) return null
        return fakeValue(c, i, table.name)
      }),
    )
  }
  return rows
}

function fakeValue(c: MockColumn, i: number, tableName: string): string | number | boolean | null {
  const t = c.type.toLowerCase()
  if (c.pk) return (1000 + i).toString().padStart(0, '0').replace(/^/, '') ? 1000 + i : 1
  if (/int|bigint|numeric|float|decimal/.test(t)) {
    if (c.name.includes('cents')) return [199, 499, 999, 1499, 2499, 4999][i % 6]
    if (c.name.includes('quantity')) return (i % 9) + 1
    if (c.name.endsWith('_id')) return 1000 + (i * 7) % 200
    return i + 1
  }
  if (/bool/.test(t)) return i % 2 === 0
  if (/time|date/.test(t)) {
    const base = new Date(Date.UTC(2026, 4, 1 + (i % 17), 8 + (i % 12), (i * 13) % 60))
    return base.toISOString().replace('T', ' ').slice(0, 19) + '+00'
  }
  if (/inet/.test(t)) return ['10.0.0.5', '172.16.4.21', '192.168.1.42', '203.0.113.7'][i % 4]
  if (/uuid/.test(t)) return `00000000-0000-4000-a000-${(1000 + i).toString().padStart(12, '0')}`
  if (c.name === 'email') {
    const names = ['alice', 'bob', 'carol', 'dan', 'evan', 'frida', 'gabe', 'hina', 'ivan', 'jess', 'kira', 'lucas']
    return `${names[i % names.length]}${i > 11 ? i : ''}@example.com`
  }
  if (c.name === 'name') {
    return ['Alice Adams', 'Bob Brown', 'Carol Chen', 'Dan Diaz', 'Evan E.', 'Frida Fischer', 'Gabe Green', 'Hina Hara', 'Ivan I.', 'Jess Jang', 'Kira K.', 'Lucas Lopez'][i % 12]
  }
  if (c.name === 'sku') return `${tableName.slice(0, 3).toUpperCase()}-${(2000 + i).toString()}`
  if (c.name === 'status') {
    return ['placed', 'paid', 'shipped', 'delivered', 'returned'][i % 5]
  }
  if (c.name === 'action') return ['auth.login', 'query.execute', 'connection.create', 'auth.logout'][i % 4]
  if (c.name === 'resource') return `conn:${(i % 3) + 1}`
  return `${c.name}-${i + 1}`
}

export const sampleSQL = `-- Recently active users with order totals
SELECT
    u.id,
    u.email,
    u.name,
    COUNT(o.id)            AS orders,
    SUM(o.total_cents) / 100.0 AS total_usd,
    MAX(o.placed_at)       AS last_order
FROM public.users u
LEFT JOIN public.orders o ON o.user_id = u.id
WHERE u.last_login_at > NOW() - INTERVAL '30 days'
GROUP BY u.id, u.email, u.name
ORDER BY total_usd DESC NULLS LAST
LIMIT 50;
`

export function mockResultFor(sql: string): MockResult {
  const trimmed = sql.trim().toLowerCase()
  if (trimmed.startsWith('select 1')) {
    return {
      columns: [{ name: 'n', typeName: 'int4' }],
      rows: [[1]],
      rowsAffected: 0,
      commandTag: 'SELECT 1',
      durationMs: 2,
      truncated: false,
    }
  }
  if (trimmed.startsWith('insert')) {
    return {
      columns: [],
      rows: [],
      rowsAffected: 1,
      commandTag: 'INSERT 0 1',
      durationMs: 4,
      truncated: false,
    }
  }
  const rows: (string | number | null)[][] = [
    [1, 'alice@example.com', 'Alice Adams', 27, 1842.55, '2026-05-16 14:21:08+00'],
    [4, 'bob@example.com', 'Bob Brown', 19, 1203.4, '2026-05-15 09:02:51+00'],
    [12, 'carol@example.com', 'Carol Chen', 14, 988.0, '2026-05-14 18:47:33+00'],
    [27, 'dan@example.com', 'Dan Diaz', 11, 742.55, '2026-05-12 11:19:12+00'],
    [33, 'evan@example.com', null, 9, 614.1, '2026-05-10 22:00:00+00'],
    [41, 'frida@example.com', 'Frida Fischer', 8, 487.6, '2026-05-09 06:15:44+00'],
    [55, 'gabe@example.com', 'Gabe Green', 7, 401.2, '2026-05-08 23:42:01+00'],
    [62, 'hina@example.com', 'Hina Hara', 7, 391.9, '2026-05-07 12:34:22+00'],
    [70, 'ivan@example.com', 'Ivan Ivanov', 6, 348.0, '2026-05-06 10:11:09+00'],
    [88, 'jess@example.com', 'Jess Jang', 5, 287.3, '2026-05-05 19:55:48+00'],
  ]
  return {
    columns: [
      { name: 'id', typeName: 'int4' },
      { name: 'email', typeName: 'text' },
      { name: 'name', typeName: 'text' },
      { name: 'orders', typeName: 'int8' },
      { name: 'total_usd', typeName: 'numeric' },
      { name: 'last_order', typeName: 'timestamptz' },
    ],
    rows,
    rowsAffected: 0,
    commandTag: `SELECT ${rows.length}`,
    durationMs: 12 + Math.floor(Math.random() * 20),
    truncated: false,
  }
}
