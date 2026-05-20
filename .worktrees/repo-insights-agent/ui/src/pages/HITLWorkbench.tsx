import { useEffect, useState, useCallback } from 'react'
import { listPendingHITL, decideHITL, type HITLRequest } from '../api/client'
import { HITLRequestCard } from '../components/HITLRequestCard'

const TENANT_ID = '00000000-0000-0000-0000-000000000001'

export function HITLWorkbench() {
  const [requests, setRequests] = useState<HITLRequest[]>([])
  const [totalCount, setTotalCount] = useState(0)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(() => {
    setError(null)
    listPendingHITL(TENANT_ID)
      .then(res => { setRequests(res.data); setTotalCount(res.total_count) })
      .catch((e: Error) => setError(e.message))
  }, [])

  useEffect(() => { load() }, [load])

  const handleDecide = async (requestId: string, decision: string, reason: string) => {
    try {
      await decideHITL(TENANT_ID, requestId, decision, 'admin', reason)
      load() // refresh list
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to submit decision')
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <h1 style={{ fontSize: 20, fontWeight: 600 }}>HITL Workbench</h1>
        <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
          <span style={{ color: '#737373', fontSize: 13 }}>{totalCount} pending</span>
          <button onClick={load} style={btnStyle}>Refresh</button>
        </div>
      </div>

      {error && <div style={{ color: '#ef4444', padding: 12, background: '#1c1c1c', borderRadius: 8, marginBottom: 16 }}>{error}</div>}

      {requests.length === 0 && !error ? (
        <div style={{ color: '#737373', padding: 32, textAlign: 'center', background: '#171717', borderRadius: 8 }}>
          No pending HITL requests
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          {requests.map(req => (
            <HITLRequestCard key={req.request_id} request={req} onDecide={handleDecide} />
          ))}
        </div>
      )}
    </div>
  )
}

const btnStyle: React.CSSProperties = {
  background: '#262626', color: '#e5e5e5', border: '1px solid #404040',
  padding: '6px 14px', borderRadius: 6, cursor: 'pointer', fontSize: 13,
}
