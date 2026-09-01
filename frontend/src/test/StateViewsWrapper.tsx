import { EmptyState, ErrorState, LoadingState } from '../components/StateViews'

export function StateViews() {
  return (
    <div>
      <LoadingState />
      <EmptyState label="Nothing here." />
      <ErrorState label="Something broke." />
    </div>
  )
}
