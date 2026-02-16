import { useEffect } from 'react'
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import { useInsightStore } from '../stores/insightStore'
import PageHeader from '../components/PageHeader'
import LoadingState from '../components/LoadingState'

const periods = [
  { value: '24h', label: '24 Hours' },
  { value: '7d', label: '7 Days' },
  { value: '30d', label: '30 Days' },
] as const

export default function Insights() {
  const {
    period,
    setPeriod,
    summary,
    timeseries,
    efficiency,
    pipeline,
    failures,
    isLoading,
    error,
    fetchAll,
  } = useInsightStore()

  useEffect(() => {
    fetchAll()
  }, [fetchAll])

  if (isLoading && !summary) {
    return (
      <div className="p-5 sm:p-6 lg:p-8">
        <LoadingState message="Loading insights..." />
      </div>
    )
  }

  return (
    <div className="p-5 sm:p-6 lg:p-8 space-y-6 animate-fade-in">
      <PageHeader
        title="Insights"
        subtitle="Token usage and cost analysis"
        action={
          <div className="flex items-center gap-1 bg-surface-hover rounded-lg p-0.5 border border-line-subtle">
            {periods.map((p) => (
              <button
                key={p.value}
                type="button"
                onClick={() => setPeriod(p.value)}
                className={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                  period === p.value
                    ? 'bg-primary-500/20 text-primary-400 shadow-sm'
                    : 'text-content-muted hover:text-content-secondary'
                }`}
              >
                {p.label}
              </button>
            ))}
          </div>
        }
      />

      {error && (
        <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-400 text-sm">
          {error}
        </div>
      )}

      {/* Token Summary */}
      {summary && (
        <div className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">Token Summary</div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            <div>
              <div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Total Tokens</div>
              <div className="text-2xl font-bold text-cyan-500 tabular-nums">{summary.total_tokens.toLocaleString()}</div>
            </div>
            <div>
              <div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Total Cost</div>
              <div className="text-2xl font-bold text-emerald-500 tabular-nums">${summary.total_cost.toFixed(2)}</div>
            </div>
            <div>
              <div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Avg Tokens/Task</div>
              <div className="text-2xl font-bold text-violet-500 tabular-nums">
                {summary.total_tasks > 0 ? Math.round(summary.total_tokens / summary.total_tasks).toLocaleString() : '0'}
              </div>
            </div>
            <div>
              <div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Cost/Task</div>
              <div className="text-2xl font-bold text-amber-500 tabular-nums">
                ${summary.total_tasks > 0 ? (summary.total_cost / summary.total_tasks).toFixed(3) : '0.000'}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Task Summary Cards */}
      {summary && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <SummaryCard label="Total Tasks" value={summary.total_tasks.toLocaleString()} color="text-content" />
          <SummaryCard
            label="Success Rate"
            value={`${summary.success_rate.toFixed(1)}%`}
            color={summary.success_rate >= 80 ? 'text-emerald-500' : summary.success_rate >= 50 ? 'text-amber-500' : 'text-rose-500'}
          />
          <SummaryCard label="Completed" value={summary.completed_tasks.toLocaleString()} color="text-emerald-500" />
          <SummaryCard label="Failed" value={summary.failed_tasks.toLocaleString()} color="text-rose-500" />
        </div>
      )}

      {/* Timeseries Chart */}
      <section className="card p-5">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">
          Task Activity Over Time
        </div>
        {timeseries.length > 0 ? (
          <ResponsiveContainer width="100%" height={280}>
            <LineChart data={timeseries}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--color-line-subtle, #333)" />
              <XAxis
                dataKey="date"
                tick={{ fontSize: 11, fill: 'var(--color-content-faint, #888)' }}
                tickFormatter={(v) => v.slice(5)}
              />
              <YAxis tick={{ fontSize: 11, fill: 'var(--color-content-faint, #888)' }} />
              <Tooltip
                contentStyle={{
                  backgroundColor: 'var(--color-surface, #1a1a1a)',
                  border: '1px solid var(--color-line, #333)',
                  borderRadius: '8px',
                  fontSize: '12px',
                }}
              />
              <Line
                type="monotone"
                dataKey="tasks_completed"
                name="Completed"
                stroke="#10b981"
                strokeWidth={2}
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="tasks_failed"
                name="Failed"
                stroke="#f43f5e"
                strokeWidth={2}
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="tasks_created"
                name="Created"
                stroke="#6366f1"
                strokeWidth={1.5}
                dot={false}
                strokeDasharray="4 4"
              />
            </LineChart>
          </ResponsiveContainer>
        ) : (
          <p className="text-content-faint italic text-sm py-8 text-center">No data for this period</p>
        )}
      </section>

      {/* Token Usage & Cost Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <section className="card p-5">
          <div className="text-sm font-semibold text-content mb-4">Token Usage</div>
          {timeseries.length > 0 ? (
            <ResponsiveContainer width="100%" height={240}>
              <AreaChart data={timeseries}>
                <defs>
                  <linearGradient id="tokenGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#818cf8" stopOpacity={0.3} />
                    <stop offset="100%" stopColor="#818cf8" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-line-subtle, #333)" />
                <XAxis
                  dataKey="date"
                  tick={{ fontSize: 10, fill: 'var(--color-content-faint, #888)' }}
                  tickFormatter={(v) => v.slice(5)}
                />
                <YAxis
                  tick={{ fontSize: 10, fill: 'var(--color-content-faint, #888)' }}
                  tickFormatter={formatTokenAxis}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: 'var(--color-surface, #1a1a1a)',
                    border: '1px solid var(--color-line, #333)',
                    borderRadius: '8px',
                    fontSize: '12px',
                  }}
                  formatter={(value) => [Number(value ?? 0).toLocaleString(), 'Tokens']}
                  labelFormatter={(label) => `Date: ${label}`}
                />
                <Area
                  type="monotone"
                  dataKey="total_tokens"
                  name="Tokens"
                  stroke="#818cf8"
                  strokeWidth={2}
                  fill="url(#tokenGrad)"
                />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <p className="text-content-faint italic text-sm py-8 text-center">No data for this period</p>
          )}
        </section>

        <section className="card p-5">
          <div className="text-sm font-semibold text-content mb-4">Cost</div>
          {timeseries.length > 0 ? (
            <ResponsiveContainer width="100%" height={240}>
              <BarChart data={timeseries}>
                <defs>
                  <linearGradient id="costGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#f59e0b" stopOpacity={0.9} />
                    <stop offset="100%" stopColor="#f59e0b" stopOpacity={0.4} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-line-subtle, #333)" />
                <XAxis
                  dataKey="date"
                  tick={{ fontSize: 10, fill: 'var(--color-content-faint, #888)' }}
                  tickFormatter={(v) => v.slice(5)}
                />
                <YAxis
                  tick={{ fontSize: 10, fill: 'var(--color-content-faint, #888)' }}
                  tickFormatter={(v) => `$${v}`}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: 'var(--color-surface, #1a1a1a)',
                    border: '1px solid var(--color-line, #333)',
                    borderRadius: '8px',
                    fontSize: '12px',
                  }}
                  formatter={(value) => [`$${Number(value ?? 0).toFixed(2)}`, 'Cost']}
                  labelFormatter={(label) => `Date: ${label}`}
                />
                <Bar dataKey="total_cost" name="Cost" fill="url(#costGrad)" radius={[3, 3, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <p className="text-content-faint italic text-sm py-8 text-center">No data for this period</p>
          )}
        </section>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Efficiency Table */}
        <section className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">
            Agent Efficiency
          </div>
          {efficiency.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-[11px] text-content-faint uppercase tracking-wider border-b border-line-subtle">
                    <th className="pb-2 pr-4">Executor</th>
                    <th className="pb-2 pr-4 text-right">Done</th>
                    <th className="pb-2 pr-4 text-right">Failed</th>
                    <th className="pb-2 pr-4 text-right">Rate</th>
                    <th className="pb-2 text-right">Avg Min</th>
                  </tr>
                </thead>
                <tbody>
                  {efficiency.map((agent) => (
                    <tr key={agent.executor_id} className="border-b border-line-subtle/50 last:border-0">
                      <td className="py-2.5 pr-4 text-content-secondary font-medium truncate max-w-[140px]">
                        {agent.executor_id}
                      </td>
                      <td className="py-2.5 pr-4 text-right text-emerald-500 tabular-nums">
                        {agent.tasks_completed}
                      </td>
                      <td className="py-2.5 pr-4 text-right text-rose-500 tabular-nums">
                        {agent.tasks_failed}
                      </td>
                      <td className="py-2.5 pr-4 text-right tabular-nums">
                        <span className={agent.success_rate >= 80 ? 'text-emerald-500' : agent.success_rate >= 50 ? 'text-amber-500' : 'text-rose-500'}>
                          {agent.success_rate.toFixed(0)}%
                        </span>
                      </td>
                      <td className="py-2.5 text-right text-content-muted tabular-nums">
                        {agent.avg_duration_min.toFixed(1)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="text-content-faint italic text-sm py-8 text-center">No executor data</p>
          )}
        </section>

        {/* Pipeline Health */}
        <section className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">
            Pipeline Health
          </div>
          {pipeline.length > 0 ? (
            <ResponsiveContainer width="100%" height={240}>
              <BarChart data={pipeline} layout="vertical">
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-line-subtle, #333)" />
                <XAxis type="number" tick={{ fontSize: 11, fill: 'var(--color-content-faint, #888)' }} />
                <YAxis
                  dataKey="status"
                  type="category"
                  width={90}
                  tick={{ fontSize: 11, fill: 'var(--color-content-faint, #888)' }}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: 'var(--color-surface, #1a1a1a)',
                    border: '1px solid var(--color-line, #333)',
                    borderRadius: '8px',
                    fontSize: '12px',
                  }}
                />
                <Bar dataKey="count" name="Tasks" fill="#6366f1" radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <p className="text-content-faint italic text-sm py-8 text-center">No pipeline data</p>
          )}
        </section>
      </div>

      {/* Failure Analysis */}
      <section className="card p-5">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">
          Failure Analysis
        </div>
        {failures.length > 0 ? (
          <div className="space-y-3">
            {failures.map((f) => (
              <div key={f.category} className="p-3 rounded-lg bg-surface-hover border border-line-subtle">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium text-content-secondary">{f.category}</span>
                  <span className="text-xs font-medium text-rose-500 tabular-nums">{f.count} failures</span>
                </div>
                {f.examples.length > 0 && (
                  <div className="space-y-1">
                    {f.examples.map((ex, i) => (
                      <p key={i} className="text-xs text-content-faint truncate">{ex}</p>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        ) : (
          <p className="text-content-faint italic text-sm py-8 text-center">No failures in this period</p>
        )}
      </section>
    </div>
  )
}

function formatTokenAxis(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}k`
  return String(value)
}

function SummaryCard({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div className="card p-4">
      <div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">{label}</div>
      <div className={`text-xl font-bold ${color} tabular-nums`}>{value}</div>
    </div>
  )
}
