import { useEffect, useMemo } from 'react'
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
  ReferenceLine,
} from 'recharts'
import { useInsightStore } from '../stores/insightStore'
import PageHeader from '../components/PageHeader'
import LoadingState from '../components/LoadingState'

const periods = [
  { value: '24h', label: '24 Hours' },
  { value: '7d', label: '7 Days' },
  { value: '30d', label: '30 Days' },
] as const

const tooltipStyle = {
  backgroundColor: 'var(--c-surface, #fff)',
  border: '1px solid var(--c-border, #e2e6f0)',
  borderRadius: '8px',
  fontSize: '12px',
  boxShadow: '0 4px 12px rgba(0,0,0,0.08)',
}

export default function Insights() {
  const {
    period,
    setPeriod,
    summary,
    timeseries,
    efficiency,
    pipeline,
    failures,
    realtimeUsage,
    projectActivities,
    billing,
    isLoading,
    error,
    fetchAll,
    startAutoRefresh,
    stopAutoRefresh,
  } = useInsightStore()

  const showCost = billing?.show_cost !== false

  useEffect(() => {
    fetchAll()
    startAutoRefresh()
    return () => stopAutoRefresh()
  }, [fetchAll, startAutoRefresh, stopAutoRefresh])

  // Hooks must be called before any early return to satisfy React rules of hooks
  const rtData = useMemo(() =>
    realtimeUsage.map((p) => ({ ...p, _ts: toEpoch(p.time) })),
    [realtimeUsage],
  )
  const tsData = useMemo(() =>
    timeseries.map((m) => ({ ...m, _ts: toEpoch(m.date) })),
    [timeseries],
  )

  if (isLoading && !summary) {
    return (
      <div className="page-shell">
        <LoadingState message="Loading insights..." />
      </div>
    )
  }

  // Compute realtime session totals from the chart data
  const rtTotalTokens = realtimeUsage.reduce((s, p) => s + p.tokens, 0)
  const rtTotalCost = realtimeUsage.reduce((s, p) => s + p.cost_usd, 0)
  const rtTotalTasks = realtimeUsage.reduce((s, p) => s + p.task_count, 0)

  return (
    <div className="page-shell space-y-6 animate-fade-in">
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
                    ? 'bg-primary-500/20 text-primary-600 shadow-sm'
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
        <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-600 text-sm">
          {error}
        </div>
      )}

      {/* Budget Status: plan-aware window or daily budget */}
      {billing && !billing.is_api && (billing.ccusage_block || billing.window_token_budget) && (() => {
        const block = billing.ccusage_block
        const tokenBudget = billing.window_token_budget ?? 5_000_000
        const tokensUsed = block?.totalTokens ?? billing.window_tokens_used ?? 0
        const blockCost = block?.totalCost ?? 0
        const blockEnd = block?.blockEnd ?? billing.window_end
        return (
          <div className="card p-4">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-3">
                <span className="text-[11px] font-medium text-content-faint uppercase tracking-widest">5h Window</span>
                <span className="text-xs text-content-muted font-mono tabular-nums">
                  {(tokensUsed / 1000).toFixed(0)}k / {(tokenBudget / 1000).toFixed(0)}k tokens
                </span>
                {blockCost > 0 && (
                  <span className="text-xs text-content-faint tabular-nums">~${blockCost.toFixed(2)}</span>
                )}
              </div>
              <div className="flex items-center gap-3">
                <div className="w-32 bg-surface-hover rounded-full h-1.5">
                  <div
                    className={`h-1.5 rounded-full transition-all ${
                      tokensUsed >= tokenBudget ? 'bg-rose-500' :
                      tokensUsed >= tokenBudget * 0.8 ? 'bg-amber-500' : 'bg-emerald-500'
                    }`}
                    style={{ width: `${Math.min(100, (tokensUsed / tokenBudget) * 100)}%` }}
                  />
                </div>
                {blockEnd && (
                  <span className="text-[10px] text-content-faint">
                    Resets {new Date(blockEnd).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
                  </span>
                )}
              </div>
            </div>
            {block?.projectedTokens && block.projectedTokens > tokensUsed && (
              <div className="text-[10px] text-content-faint">
                Projected: {(block.projectedTokens / 1000).toFixed(0)}k tokens (~${(block.projectedCost ?? 0).toFixed(2)}) by window end
              </div>
            )}
          </div>
        )
      })()}

      {/* Unified Summary: Period stats + Realtime session */}
      {summary && (
        <div className="card p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest">Usage Summary</div>
              {billing && !billing.is_api && (
                <span className="text-[10px] text-content-faint bg-surface-hover px-1.5 py-0.5 rounded">{billing.plan}</span>
              )}
            </div>
            <div className="flex items-center gap-1.5">
              <div className="w-1.5 h-1.5 rounded-full bg-emerald-400 shadow-sm shadow-emerald-500/30 animate-glow-pulse" />
              <span className="text-[10px] text-content-faint">Live</span>
            </div>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            <div>
              <div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Total Tokens</div>
              <div className="text-2xl font-bold text-cyan-500 tabular-nums">{summary.total_tokens.toLocaleString()}</div>
              {rtTotalTokens > 0 && <div className="text-[10px] text-cyan-400/70 mt-0.5 tabular-nums">+{rtTotalTokens.toLocaleString()} last 60m</div>}
            </div>
            <div>
              <div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">
                {showCost ? 'Total Cost' : 'Est. Cost'}
                {!showCost && <span className="normal-case tracking-normal font-normal"> (ccusage)</span>}
              </div>
              {showCost ? (
                <>
                  <div className="text-2xl font-bold text-emerald-500 tabular-nums">${summary.total_cost.toFixed(2)}</div>
                  {rtTotalCost > 0 && <div className="text-[10px] text-emerald-400/70 mt-0.5 tabular-nums">+${rtTotalCost.toFixed(4)} last 60m</div>}
                </>
              ) : (
                <>
                  <div className="text-2xl font-bold text-content-muted tabular-nums">~${(billing?.ccusage_daily?.totalCost ?? 0).toFixed(2)}</div>
                  <div className="text-[10px] text-content-faint mt-0.5">calculated from model rates</div>
                </>
              )}
            </div>
            <div>
              <div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">Avg Tokens/Task</div>
              <div className="flex items-baseline gap-2">
                <div className="text-lg font-bold text-violet-500 tabular-nums">
                  {summary.total_tasks > 0 ? Math.round(summary.execution_tokens / summary.total_tasks).toLocaleString() : '0'}
                </div>
                <span className="text-[10px] text-content-faint">exec</span>
              </div>
              <div className="flex items-baseline gap-2">
                <div className="text-lg font-bold text-violet-400/70 tabular-nums">
                  {summary.total_tasks > 0 ? Math.round(summary.triage_tokens / summary.total_tasks).toLocaleString() : '0'}
                </div>
                <span className="text-[10px] text-content-faint">triage</span>
              </div>
            </div>
            <div>
              <div className="text-[10px] text-content-faint uppercase tracking-wider mb-1">
                {showCost ? 'Cost/Task' : 'Est. Cost/Task'}
              </div>
              {showCost ? (
                <>
                  <div className="flex items-baseline gap-2">
                    <div className="text-lg font-bold text-amber-500 tabular-nums">
                      ${summary.total_tasks > 0 ? (summary.execution_cost / summary.total_tasks).toFixed(3) : '0.000'}
                    </div>
                    <span className="text-[10px] text-content-faint">exec</span>
                  </div>
                  <div className="flex items-baseline gap-2">
                    <div className="text-lg font-bold text-amber-400/70 tabular-nums">
                      ${summary.total_tasks > 0 ? (summary.triage_cost / summary.total_tasks).toFixed(3) : '0.000'}
                    </div>
                    <span className="text-[10px] text-content-faint">triage</span>
                  </div>
                </>
              ) : (
                <>
                  <div className="text-lg font-bold text-content-muted tabular-nums">
                    ~${summary.total_tasks > 0 ? ((billing?.ccusage_daily?.totalCost ?? 0) / summary.total_tasks).toFixed(3) : '0.000'}
                  </div>
                  <div className="text-[10px] text-content-faint mt-0.5">per ccusage</div>
                </>
              )}
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

      {/* Activity Heatmap & Project Activity */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <ActivityHeatmap timeseries={timeseries} />
        <ProjectActivityChart projectActivities={projectActivities} />
      </div>

      {/* Token Usage & Daily Task Volume */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <section className="card p-5">
          <div className="text-sm font-semibold text-content mb-4">Token Usage</div>
          {tsData.length > 0 ? (
            <ResponsiveContainer width="100%" height={240}>
              <AreaChart data={tsData}>
                <defs>
                  <linearGradient id="tokenGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#818cf8" stopOpacity={0.3} />
                    <stop offset="100%" stopColor="#818cf8" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--c-border-subtle, #edf0f7)" />
                <XAxis
                  dataKey="_ts"
                  type="number"
                  scale="time"
                  domain={['dataMin', 'dataMax']}
                  tick={{ fontSize: 10, fill: 'var(--c-text-faint, #b5bbce)' }}
                  tickFormatter={(v) => formatEpochDate(v, period)}
                />
                <YAxis
                  tick={{ fontSize: 10, fill: 'var(--c-text-faint, #b5bbce)' }}
                  tickFormatter={formatTokenAxis}
                />
                <Tooltip
                  contentStyle={tooltipStyle}
                  formatter={(value) => [Number(value ?? 0).toLocaleString(), 'Tokens']}
                  labelFormatter={(label) => formatEpochLabel(Number(label))}
                />
                <Area
                  type="monotoneX"
                  dataKey="total_tokens"
                  name="Tokens"
                  stroke="#818cf8"
                  strokeWidth={2}
                  fill="url(#tokenGrad)"
                  dot={false}
                />
                <ReferenceLine
                  y={tsData.length > 0 ? tsData.reduce((s, d) => s + d.total_tokens, 0) / tsData.length : 0}
                  stroke="#a78bfa"
                  strokeDasharray="6 3"
                  strokeWidth={1.5}
                  label={{ value: 'avg', position: 'right', fontSize: 10, fill: '#a78bfa' }}
                />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <p className="text-content-faint italic text-sm py-8 text-center">No data for this period</p>
          )}
        </section>

        <DailyTaskVolumeChart timeseries={timeseries} period={period} />
      </div>

      {/* Realtime Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <section className="card p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest">
              Realtime Tokens
            </div>
            <div className="flex items-center gap-3">
              {rtTotalTasks > 0 && <span className="text-[10px] text-content-faint tabular-nums">{rtTotalTasks} tasks</span>}
              <div className="flex items-center gap-1.5">
                <div className="w-1.5 h-1.5 rounded-full bg-emerald-400 shadow-sm shadow-emerald-500/30 animate-glow-pulse" />
                <span className="text-[10px] text-content-faint">Last 60 min</span>
              </div>
            </div>
          </div>
          {rtData.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <AreaChart data={rtData}>
                <defs>
                  <linearGradient id="rtTokenGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#818cf8" stopOpacity={0.3} />
                    <stop offset="100%" stopColor="#818cf8" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--c-border-subtle, #edf0f7)" />
                <XAxis
                  dataKey="_ts"
                  type="number"
                  scale="time"
                  domain={['dataMin', 'dataMax']}
                  tick={{ fontSize: 10, fill: 'var(--c-text-faint, #b5bbce)' }}
                  tickFormatter={formatEpochTime}
                  axisLine={{ stroke: 'var(--c-border-subtle, #edf0f7)' }}
                  tickLine={false}
                />
                <YAxis
                  tick={{ fontSize: 10, fill: 'var(--c-text-faint, #b5bbce)' }}
                  tickFormatter={formatTokenAxis}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip
                  contentStyle={tooltipStyle}
                  formatter={(value) => [Number(value ?? 0).toLocaleString(), 'Tokens']}
                  labelFormatter={(label) => formatEpochLabel(Number(label))}
                />
                <Area
                  type="monotoneX"
                  dataKey="tokens"
                  name="Tokens"
                  stroke="#818cf8"
                  strokeWidth={2}
                  fill="url(#rtTokenGrad)"
                  dot={false}
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <p className="text-content-faint italic text-sm py-8 text-center">No realtime data yet</p>
          )}
        </section>

        <section className="card p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest">
              Realtime Cost
            </div>
            <div className="flex items-center gap-3">
              {rtTotalCost > 0 && <span className="text-[10px] text-content-faint tabular-nums">${rtTotalCost.toFixed(4)}</span>}
              <div className="flex items-center gap-1.5">
                <div className="w-1.5 h-1.5 rounded-full bg-emerald-400 shadow-sm shadow-emerald-500/30 animate-glow-pulse" />
                <span className="text-[10px] text-content-faint">Last 60 min</span>
              </div>
            </div>
          </div>
          {rtData.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <AreaChart data={rtData}>
                <defs>
                  <linearGradient id="rtCostGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#f59e0b" stopOpacity={0.3} />
                    <stop offset="100%" stopColor="#f59e0b" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--c-border-subtle, #edf0f7)" />
                <XAxis
                  dataKey="_ts"
                  type="number"
                  scale="time"
                  domain={['dataMin', 'dataMax']}
                  tick={{ fontSize: 10, fill: 'var(--c-text-faint, #b5bbce)' }}
                  tickFormatter={formatEpochTime}
                  axisLine={{ stroke: 'var(--c-border-subtle, #edf0f7)' }}
                  tickLine={false}
                />
                <YAxis
                  tick={{ fontSize: 10, fill: 'var(--c-text-faint, #b5bbce)' }}
                  tickFormatter={(v) => `$${v}`}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip
                  contentStyle={tooltipStyle}
                  formatter={(value) => [`$${Number(value ?? 0).toFixed(4)}`, 'Cost']}
                  labelFormatter={(label) => formatEpochLabel(Number(label))}
                />
                <Area
                  type="monotoneX"
                  dataKey="cost_usd"
                  name="Cost"
                  stroke="#f59e0b"
                  strokeWidth={2}
                  fill="url(#rtCostGrad)"
                  dot={false}
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <p className="text-content-faint italic text-sm py-8 text-center">No realtime data yet</p>
          )}
        </section>
      </div>

      {/* Timeseries Chart */}
      <section className="card p-5">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">
          Task Activity Over Time
        </div>
        {tsData.length > 0 ? (
          <ResponsiveContainer width="100%" height={280}>
            <LineChart data={tsData}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--c-border-subtle, #edf0f7)" />
              <XAxis
                dataKey="_ts"
                type="number"
                scale="time"
                domain={['dataMin', 'dataMax']}
                tick={{ fontSize: 11, fill: 'var(--c-text-faint, #b5bbce)' }}
                tickFormatter={(v) => formatEpochDate(v, period)}
              />
              <YAxis tick={{ fontSize: 11, fill: 'var(--c-text-faint, #b5bbce)' }} />
              <Tooltip
                contentStyle={tooltipStyle}
                labelFormatter={(label) => formatEpochLabel(Number(label))}
              />
              <Line
                type="monotoneX"
                dataKey="tasks_completed"
                name="Completed"
                stroke="#10b981"
                strokeWidth={2}
                dot={false}
              />
              <Line
                type="monotoneX"
                dataKey="tasks_failed"
                name="Failed"
                stroke="#f43f5e"
                strokeWidth={2}
                dot={false}
              />
              <Line
                type="monotoneX"
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
                <CartesianGrid strokeDasharray="3 3" stroke="var(--c-border-subtle, #edf0f7)" />
                <XAxis type="number" tick={{ fontSize: 11, fill: 'var(--c-text-faint, #b5bbce)' }} />
                <YAxis
                  dataKey="status"
                  type="category"
                  width={90}
                  tick={{ fontSize: 11, fill: 'var(--c-text-faint, #b5bbce)' }}
                />
                <Tooltip contentStyle={tooltipStyle} />
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

/* ─── Activity Heatmap (GitHub-style 7-row × N-col grid) ─── */
const DAY_LABELS = ['Sun', '', 'Tue', '', 'Thu', '', 'Sat']

function ActivityHeatmap({ timeseries }: { timeseries: import('../lib/api').DailyMetric[] }) {
  const { weeks, totalTasks, activeDays, monthLabels } = useMemo(() => {
    const dateMap = new Map<string, number>()
    for (const d of timeseries) {
      dateMap.set(d.date.slice(0, 10), d.tasks_created)
    }

    // Go back far enough to fill complete weeks (up to ~13 weeks / 90 days)
    const today = new Date()
    const todayDow = today.getDay() // 0=Sun
    // Start from the Sunday of the earliest week we want to show
    const daysBack = 12 * 7 + todayDow // 12 full weeks + partial current week
    const start = new Date(today)
    start.setDate(start.getDate() - daysBack)

    // Build column-major grid: each column is a week (Sun→Sat)
    const cols: { date: string; count: number; dayOfWeek: number; future: boolean }[][] = []
    let total = 0
    let active = 0
    const mLabels: { col: number; label: string }[] = []
    let lastMonth = -1

    const cursor = new Date(start)
    let col: typeof cols[0] = []
    while (cursor <= today || col.length > 0) {
      const isFuture = cursor > today
      const key = cursor.toISOString().slice(0, 10)
      const count = isFuture ? -1 : (dateMap.get(key) ?? 0)
      if (!isFuture) {
        total += count
        if (count > 0) active++
      }

      // Track month boundaries for labels
      const m = cursor.getMonth()
      if (m !== lastMonth && !isFuture) {
        mLabels.push({ col: cols.length, label: cursor.toLocaleDateString(undefined, { month: 'short' }) })
        lastMonth = m
      }

      col.push({ date: key, count, dayOfWeek: cursor.getDay(), future: isFuture })

      if (col.length === 7) {
        cols.push(col)
        col = []
      }
      cursor.setDate(cursor.getDate() + 1)
      if (isFuture && col.length === 0) break
    }
    if (col.length > 0) cols.push(col)

    return { weeks: cols, totalTasks: total, activeDays: active, monthLabels: mLabels }
  }, [timeseries])

  const getColor = (count: number) => {
    if (count < 0) return 'bg-transparent' // future
    if (count === 0) return 'bg-emerald-500/[0.06]'
    if (count <= 2) return 'bg-emerald-500/25'
    if (count <= 5) return 'bg-emerald-500/45'
    if (count <= 10) return 'bg-emerald-500/70'
    return 'bg-emerald-500'
  }

  return (
    <section className="card p-5">
      <div className="flex items-center justify-between mb-1">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest">
          Activity Heatmap
        </div>
      </div>
      <div className="text-xs text-content-muted mb-3">
        {totalTasks} tasks in {activeDays} active days
      </div>
      <div className="overflow-x-auto">
        {/* Month labels */}
        <div className="flex ml-[26px] mb-1">
          {monthLabels.map((m, i) => (
            <div
              key={i}
              className="text-[10px] text-content-faint"
              style={{ position: 'relative', left: m.col * 15 - (i > 0 ? monthLabels[i - 1].col * 15 + (monthLabels[i - 1].label.length * 6) : 0) }}
            >
              {m.label}
            </div>
          ))}
        </div>
        <div className="flex gap-[3px]">
          {/* Day-of-week labels */}
          <div className="flex flex-col gap-[3px] mr-[3px] shrink-0">
            {DAY_LABELS.map((label, i) => (
              <div key={i} className="h-[13px] flex items-center">
                <span className="text-[9px] text-content-faint leading-none w-[20px]">{label}</span>
              </div>
            ))}
          </div>
          {/* Week columns */}
          {weeks.map((week, wi) => (
            <div key={wi} className="flex flex-col gap-[3px]">
              {week.map((cell) => (
                <div
                  key={cell.date}
                  className={`w-[13px] h-[13px] rounded-[2px] ${getColor(cell.count)}`}
                  title={cell.future ? '' : `${cell.date}: ${cell.count} tasks`}
                />
              ))}
            </div>
          ))}
        </div>
      </div>
      <div className="flex items-center gap-1.5 mt-3 justify-end">
        <span className="text-[10px] text-content-faint">Less</span>
        {[0, 1, 3, 6, 11].map((v) => (
          <div key={v} className={`w-[10px] h-[10px] rounded-[2px] ${getColor(v)}`} />
        ))}
        <span className="text-[10px] text-content-faint">More</span>
      </div>
    </section>
  )
}

/* ─── Project Activity (horizontal bar chart) ─── */
function ProjectActivityChart({ projectActivities }: { projectActivities: import('../lib/api').ProjectActivity[] }) {
  const data = useMemo(
    () => projectActivities
      .slice()
      .sort((a, b) => b.task_count - a.task_count)
      .slice(0, 10)
      .map((p) => ({ name: p.project_name || p.project_id, tasks: p.task_count })),
    [projectActivities],
  )

  return (
    <section className="card p-5">
      <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-4">
        Project Activity
      </div>
      {data.length > 0 ? (
        <ResponsiveContainer width="100%" height={240}>
          <BarChart data={data} layout="vertical" margin={{ left: 4, right: 16 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--c-border-subtle, #edf0f7)" horizontal={false} />
            <XAxis type="number" tick={{ fontSize: 10, fill: 'var(--c-text-faint, #b5bbce)' }} />
            <YAxis
              dataKey="name"
              type="category"
              width={100}
              tick={{ fontSize: 10, fill: 'var(--c-text-faint, #b5bbce)' }}
            />
            <Tooltip contentStyle={tooltipStyle} />
            <Bar dataKey="tasks" name="Tasks" fill="#8b5cf6" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <p className="text-content-faint italic text-sm py-8 text-center">No project data</p>
      )}
    </section>
  )
}

/* ─── Daily Task Volume (bar chart) ─── */
function DailyTaskVolumeChart({ timeseries, period }: { timeseries: import('../lib/api').DailyMetric[]; period: string }) {
  const { data, totalTasks, avgPerDay, activeDays } = useMemo(() => {
    const mapped = timeseries.map((m) => ({
      _ts: toEpoch(m.date),
      tasks: m.tasks_created,
    }))
    const total = mapped.reduce((s, d) => s + d.tasks, 0)
    const active = mapped.filter((d) => d.tasks > 0).length
    return {
      data: mapped,
      totalTasks: total,
      avgPerDay: mapped.length > 0 ? (total / mapped.length).toFixed(1) : '0',
      activeDays: active,
    }
  }, [timeseries])

  return (
    <section className="card p-5">
      <div className="flex items-center justify-between mb-1">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest">
          Daily Task Volume
        </div>
      </div>
      <div className="flex items-center gap-4 text-xs text-content-muted mb-4">
        <span>{totalTasks} total</span>
        <span>{avgPerDay} avg/day</span>
        <span>{activeDays} active days</span>
      </div>
      {data.length > 0 ? (
        <ResponsiveContainer width="100%" height={200}>
          <BarChart data={data}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--c-border-subtle, #edf0f7)" />
            <XAxis
              dataKey="_ts"
              type="number"
              scale="time"
              domain={['dataMin', 'dataMax']}
              tick={{ fontSize: 10, fill: 'var(--c-text-faint, #b5bbce)' }}
              tickFormatter={(v) => formatEpochDate(v, period)}
            />
            <YAxis tick={{ fontSize: 10, fill: 'var(--c-text-faint, #b5bbce)' }} allowDecimals={false} />
            <Tooltip
              contentStyle={tooltipStyle}
              formatter={(value) => [Number(value ?? 0), 'Tasks']}
              labelFormatter={(label) => formatEpochLabel(Number(label))}
            />
            <Bar dataKey="tasks" name="Tasks Created" fill="#f59e0b" radius={[3, 3, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <p className="text-content-faint italic text-sm py-8 text-center">No data for this period</p>
      )}
    </section>
  )
}

// Convert date/time strings to epoch ms for continuous axis
function toEpoch(v: string): number {
  if (!v) return 0
  // Full ISO: "2026-02-17T14:00:00Z"
  if (v.includes('T')) return new Date(v).getTime()
  // Date-space-time: "2026-02-17 14:00"
  if (v.includes(' ')) return new Date(v.replace(' ', 'T') + ':00Z').getTime()
  // Date only: "2026-02-17"
  return new Date(v + 'T00:00:00Z').getTime()
}

// Format epoch ms for continuous x-axis ticks (time-of-day)
function formatEpochTime(ts: number): string {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
}

// Format epoch ms for continuous x-axis ticks (date or date+time)
function formatEpochDate(ts: number, period: string): string {
  if (!ts) return ''
  const d = new Date(ts)
  if (period === '24h') return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
  if (period === '7d') return d.toLocaleDateString(undefined, { month: 'numeric', day: 'numeric' })
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

// Format epoch ms for tooltip labels
function formatEpochLabel(ts: number): string {
  if (!ts) return ''
  return new Date(ts).toLocaleString()
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
