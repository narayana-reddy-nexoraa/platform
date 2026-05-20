import { useEffect, useState, useCallback } from 'react'
import { listSOPExecutions, type SOPExecution, type PaginatedResponse } from '../api/client'
import { SOPExecutionList } from '../components/SOPExecutionList'

const TENANT_ID = '00000000-0000-0000-0000-000000000001' // dev default

export function Dashboard() {
  const [data, setData] = useState<PaginatedResponse<SOPExecution> | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState('')
  const [industryFilter, setIndustryFilter] = useState('')

  const load = useCallback(() => {
    setError(null)
    listSOPExecutions(TENANT_ID, {
      status: statusFilter || undefined,
      industry: industryFilter || undefined,
      limit: 50,
    })
      .then(setData)
      .catch((e: Error) => setError(e.message))
  }, [statusFilter, industryFilter])

  useEffect(() => { load() }, [load])

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <h1 style={{ fontSize: 20, fontWeight: 600 }}>SOP Executions</h1>
        <button onClick={load} style={btnStyle}>Refresh</button>
      </div>

      <div style={{ display: 'flex', gap: 12, marginBottom: 16 }}>
        <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)} style={selectStyle}>
          <option value="">All statuses</option>
          <option value="PENDING">Pending</option>
          <option value="RUNNING">Running</option>
          <option value="WAITING_HITL">Waiting HITL</option>
          <option value="COMPLETED">Completed</option>
          <option value="FAILED">Failed</option>
          <option value="ESCALATED">Escalated</option>
        </select>
        <select value={industryFilter} onChange={e => setIndustryFilter(e.target.value)} style={selectStyle}>
          <option value="">All industries</option>
          <option value="FINANCIAL_SERVICES">Financial Services</option>
          <option value="INSURANCE">Insurance</option>
          <option value="HEALTHCARE">Healthcare</option>
          <option value="HOSPITAL_OPS">Hospital Ops</option>
          <option value="LIFE_SCIENCES">Life Sciences</option>
          <option value="MANUFACTURING">Manufacturing</option>
        </select>
      </div>

      {error && <div style={{ color: '#ef4444', padding: 12, background: '#1c1c1c', borderRadius: 8, marginBottom: 16 }}>{error}</div>}

      {data ? (
        <>
          <div style={{ color: '#737373', fontSize: 13, marginBottom: 8 }}>{data.total_count} total executions</div>
          <SOPExecutionList executions={data.data} />
        </>
      ) : !error ? (
        <div style={{ color: '#737373' }}>Loading...</div>
      ) : null}
    </div>
  )
}

const btnStyle: React.CSSProperties = {
  background: '#262626', color: '#e5e5e5', border: '1px solid #404040',
  padding: '6px 14px', borderRadius: 6, cursor: 'pointer', fontSize: 13,
}
const selectStyle: React.CSSProperties = {
  background: '#171717', color: '#e5e5e5', border: '1px solid #404040',
  padding: '6px 10px', borderRadius: 6, fontSize: 13,
}
