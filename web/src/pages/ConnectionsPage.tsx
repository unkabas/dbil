import { mockConnections, tablesFor } from '../mock/data'
import TagBadge from '../components/TagBadge'
import Icon from '../components/Icon'

export default function ConnectionsPage() {
  return (
    <div className="h-full overflow-auto bg-app-grad">
      <div className="max-w-[1100px] mx-auto px-6 py-6">
        <header className="flex items-end justify-between mb-5">
          <div>
            <h1 className="text-[22px] font-semibold text-ink-50 tracking-tight">Connections</h1>
            <p className="text-ink-300 text-[13px] mt-0.5">
              {mockConnections.length} registered Postgres databases
            </p>
          </div>
          <button className="h-9 px-4 rounded-md bg-violet text-white font-medium text-[13px] flex items-center gap-2 hover:bg-violet-deep transition-colors shadow-glow">
            <Icon name="plus" className="w-3.5 h-3.5" />
            <span>Add connection</span>
          </button>
        </header>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {mockConnections.map((c) => {
            const tableCount = tablesFor(c.id).length
            return (
              <div
                key={c.id}
                className="bg-ink-800/70 backdrop-blur-sm border border-ink-700 rounded-xl shadow-card overflow-hidden hover:border-ink-600 transition-colors"
              >
                <div className="bg-header-grad px-5 py-4 border-b border-ink-700 flex items-start justify-between gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-ink-50 text-[15px]">{c.alias}</span>
                      <TagBadge tag={c.tag} size="xs" />
                    </div>
                    <div className="font-mono text-ink-300 text-[12px] mt-1">
                      {c.host}:{c.port}/{c.database}
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <IconBtn title="Test connection" name="refresh" />
                    <IconBtn title="Edit" name="pencil" />
                    <IconBtn title="Delete" name="trash" />
                  </div>
                </div>

                <dl className="px-5 py-3 grid grid-cols-3 gap-2 text-[12.5px]">
                  <Field label="TLS" value={c.tls_mode} />
                  <Field label="Tables" value={String(tableCount)} />
                  <Field label="Tag" value={c.tag} />
                </dl>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-ink-400 text-[10.5px] uppercase tracking-wider">{label}</dt>
      <dd className="text-ink-100 font-mono">{value}</dd>
    </div>
  )
}

function IconBtn({ title, name }: { title: string; name: 'refresh' | 'pencil' | 'trash' }) {
  return (
    <button
      title={title}
      className="p-1.5 rounded-md hover:bg-ink-700 text-ink-300 hover:text-ink-50"
    >
      <Icon name={name} className="w-3.5 h-3.5" />
    </button>
  )
}
