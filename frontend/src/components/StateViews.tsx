export function LoadingState({ label = 'Loading…' }: { label?: string }) {
  return (
    <div className="state state-loading" role="status">
      <span className="spinner" aria-hidden="true" />
      <span>{label}</span>
    </div>
  )
}

export function EmptyState({ label }: { label: string }) {
  return (
    <div className="state state-empty" role="status">
      <span>{label}</span>
    </div>
  )
}

export function ErrorState({ label, onRetry }: { label: string; onRetry?: () => void }) {
  return (
    <div className="state state-error" role="alert">
      <span>{label}</span>
      {onRetry && (
        <button type="button" onClick={onRetry} className="link-button">
          Retry
        </button>
      )}
    </div>
  )
}
