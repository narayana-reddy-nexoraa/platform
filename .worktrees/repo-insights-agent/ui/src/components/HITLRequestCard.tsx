import { useState } from 'react'
import type { HITLRequest } from '../api/client'

interface Props {
  request: HITLRequest
  onDecide: (requestId: string, decision: string, reason: string) => Promise<void>
}

export function HITLRequestCard({ request, onDecide }: Props) {
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleDecision = async (decision: string) => {
    setSubmitting(true)
    try {
      await onDecide(request.request_id, decision, reason)
    } finally {
      setSubmitting(false)
    }
  }

  const deadline = new Date(request.deadline)
  const isOverdue = request.is_overdue

  return (
    <div style={{
      background: '#171717', border: `1px solid ${isOverdue ? '#dc2626' : '#262626'}`,
      borderRadius: 8, padding: 16,
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 12 }}>
        <div>
          <div style={{ fontWeight: 600, marginBottom: 4 }}>{request.step_name}</div>
          <div style={{ fontSize: 12, color: '#737373' }}>
            SOP: <span style={{ fontFamily: 'monospace' }}>{request.sop_id}</span>
            {' · '}Step: <span style={{ fontFamily: 'monospace' }}>{request.step_id}</span>
          </div>
        </div>
        <div style={{ textAlign: 'right' }}>
          {isOverdue ? (
            <span style={{ color: '#ef4444', fontSize: 12, fontWeight: 600 }}>OVERDUE</span>
          ) : (
            <span style={{ color: '#737373', fontSize: 12 }}>Due: {deadline.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
          )}
        </div>
      </div>

      <div style={{ marginBottom: 12 }}>
        <input
          type="text"
          placeholder="Reason (optional)"
          value={reason}
          onChange={e => setReason(e.target.value)}
          disabled={submitting}
          style={{
            width: '100%', padding: '8px 10px', background: '#0a0a0a', border: '1px solid #404040',
            borderRadius: 6, color: '#e5e5e5', fontSize: 13,
          }}
        />
      </div>

      <div style={{ display: 'flex', gap: 8 }}>
        <button
          onClick={() => handleDecision('APPROVED')}
          disabled={submitting}
          style={{ ...actionBtn, background: '#166534', borderColor: '#22c55e' }}
        >
          Approve
        </button>
        <button
          onClick={() => handleDecision('REJECTED')}
          disabled={submitting}
          style={{ ...actionBtn, background: '#7f1d1d', borderColor: '#ef4444' }}
        >
          Reject
        </button>
        <button
          onClick={() => handleDecision('ESCALATED')}
          disabled={submitting}
          style={{ ...actionBtn, background: '#78350f', borderColor: '#f97316' }}
        >
          Escalate
        </button>
      </div>
    </div>
  )
}

const actionBtn: React.CSSProperties = {
  padding: '6px 16px', borderRadius: 6, border: '1px solid',
  color: '#e5e5e5', cursor: 'pointer', fontSize: 13, fontWeight: 500,
}
