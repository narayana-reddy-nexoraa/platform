import { useEffect, useState, useCallback } from 'react'
import { listSOPExecutions, type SOPExecution, type PaginatedResponse } from '../api/client'

const TENANT_ID = '00000000-0000-0000-0000-000000000001'

interface StatusCount { status: string; count: number; color: string }
interface IndustryCount { industry: string; count: number }

export function Analytics() {
  const [data, setData] = useState<PaginatedResponse<SOPExecution> | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(() => {
    setError(null)
    listSOPExecutions(TENANT_ID, { limit: 100 })
      .then(setData)
      .catch((e: Error) => setError(e.message))
  }, [])

  useEffect(() => { load() }, [load])

  if (error) return <div style={{ color: '#ef4444' }}>{error}</div>
  if (!data) return <div style={{ color: '#737373' }}>Loading analytics...</div>

  const statusCounts = computeStatusCounts(data.data)
  const industryCounts = computeIndustryCounts(data.data)
  const avgLatency = computeAvgDuration(data.data)

  return (
    <div>
      <h1 style={{ fontSize: 20, fontWeight: 600, marginBottom: 20 }}>Analytics</h1>

      {/* KPI Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 12, marginBottom: 24 }}>
        <KPICard label="Total Executions" value={data.total_count} />
        <KPICard label="Completed" value={statusCounts.find(s => s.status === 'COMPLETED')?.count ?? 0} color="#22c55e" />
        <KPICard label="Failed" value={statusCounts.find(s => s.status === 'FAILED')?.count ?? 0} color="#ef4444" />
        <KPICard label="Waiting HITL" value={statusCounts.find(s => s.status === 'WAITING_HITL')?.count ?? 0} color="#eab308" />
        <KPICard label="Avg Duration" value={avgLatency ? `${avgLatency}s` : 'N/A'} />
      </div>

      {/* Status Breakdown */}
      <h2 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12 }}>By Status</h2>
      <div style={{ display: 'flex', gap: 8, marginBottom: 24, flexWrap: 'wrap' }}>
        {statusCounts.map(s => (
          <div key={s.status} style={{ background: '#171717', border: '1px solid #262626', borderRadius: 8, padding: '10px 16px', minWidth: 120 }}>
            <div style={{ fontSize: 12, color: '#737373', marginBottom: 4 }}>{s.status}</div>
            <div style={{ fontSize: 20, fontWeight: 700, color: s.color }}>{s.count}</div>
          </div>
        ))}
      </div>

      {/* Industry Breakdown */}
      <h2 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12 }}>By Industry</h2>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        {industryCounts.map(i => (
          <div key={i.industry} style={{ background: '#171717', border: '1px solid #262626', borderRadius: 8, padding: '10px 16px', minWidth: 140 }}>
            <div style={{ fontSize: 12, color: '#737373', marginBottom: 4 }}>{i.industry.replace(/_/g, ' ')}</div>
            <div style={{ fontSize: 20, fontWeight: 700 }}>{i.count}</div>
          </div>
        ))}
      </div>
    </div>
  )
}

function KPICard({ label, value, color }: { label: string; value: number | string; color?: string }) {
  return (
    <div style={{ background: '#171717', border: '1px solid #262626', borderRadius: 8, padding: 16 }}>
      <div style={{ fontSize: 12, color: '#737373', marginBottom: 6 }}>{label}</div>
      <div style={{ fontSize: 24, fontWeight: 700, color: color || '#e5e5e5' }}>{value}</div>
    </div>
  )
}

function computeStatusCounts(executions: SOPExecution[]): StatusCount[] {
  const colorMap: Record<string, string> = {
    PENDING: '#a3a3a3', RUNNING: '#3b82f6', WAITING_HITL: '#eab308',
    COMPLETED: '#22c55e', FAILED: '#ef4444', ESCALATED: '#f97316', CANCELED: '#6b7280',
  }
  const counts: Record<string, number> = {}
  for (const e of executions) counts[e.status] = (counts[e.status] || 0) + 1
  return Object.entries(counts).map(([status, count]) => ({ status, count, color: colorMap[status] || '#a3a3a3' }))
}

function computeIndustryCounts(executions: SOPExecution[]): IndustryCount[] {
  const counts: Record<string, number> = {}
  for (const e of executions) counts[e.industry] = (counts[e.industry] || 0) + 1
  return Object.entries(counts).map(([industry, count]) => ({ industry, count })).sort((a, b) => b.count - a.count)
}

function computeAvgDuration(executions: SOPExecution[]): number | null {
  const completed = executions.filter(e => e.status === 'COMPLETED' && e.completed_at && e.started_at)
  if (completed.length === 0) return null
  const total = completed.reduce((sum, e) => {
    return sum + (new Date(e.completed_at!).getTime() - new Date(e.started_at).getTime()) / 1000
  }, 0)
  return Math.round(total / completed.length)
}
