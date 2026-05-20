import type { SOPExecution } from '../api/client'

interface Props {
  executions: SOPExecution[]
}

const STATUS_COLORS: Record<string, string> = {
  PENDING: '#a3a3a3', RUNNING: '#3b82f6', WAITING_HITL: '#eab308',
  COMPLETED: '#22c55e', FAILED: '#ef4444', ESCALATED: '#f97316', CANCELED: '#6b7280',
}

export function SOPExecutionList({ executions }: Props) {
  if (executions.length === 0) {
    return <div style={{ color: '#737373', padding: 32, textAlign: 'center', background: '#171717', borderRadius: 8 }}>No executions found</div>
  }

  return (
    <div style={{ overflowX: 'auto' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
        <thead>
          <tr style={{ borderBottom: '1px solid #262626' }}>
            {['SOP ID', 'Industry', 'Status', 'Current Step', 'Started', 'Duration'].map(h => (
              <th key={h} style={{ padding: '8px 12px', textAlign: 'left', color: '#737373', fontWeight: 500 }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {executions.map(exec => (
            <tr key={exec.sop_execution_id} style={{ borderBottom: '1px solid #1c1c1c' }}>
              <td style={cellStyle}>
                <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{exec.sop_id}</span>
              </td>
              <td style={cellStyle}>{exec.industry.replace(/_/g, ' ')}</td>
              <td style={cellStyle}>
                <StatusBadge status={exec.status} />
              </td>
              <td style={cellStyle}>
                <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{exec.current_step}</span>
              </td>
              <td style={cellStyle}>{formatTime(exec.started_at || exec.created_at)}</td>
              <td style={cellStyle}>{formatDuration(exec.started_at, exec.completed_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  const color = STATUS_COLORS[status] || '#a3a3a3'
  return (
    <span style={{
      display: 'inline-block', padding: '2px 8px', borderRadius: 4, fontSize: 11, fontWeight: 600,
      background: `${color}20`, color, border: `1px solid ${color}40`,
    }}>
      {status}
    </span>
  )
}

function formatTime(iso: string): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function formatDuration(start?: string, end?: string): string {
  if (!start) return '—'
  const s = new Date(start).getTime()
  const e = end ? new Date(end).getTime() : Date.now()
  const sec = Math.round((e - s) / 1000)
  if (sec < 60) return `${sec}s`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`
  return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`
}

const cellStyle: React.CSSProperties = { padding: '10px 12px' }
