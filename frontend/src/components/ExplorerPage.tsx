import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getAsset } from '../api/client'
import { useAssetTree } from '../hooks/useAssetTree'
import { AssetTreeView } from './AssetTreeView'
import { AssetDetailsPanel } from './AssetDetailsPanel'
import { SearchBox } from './SearchBox'
import { EmptyState, ErrorState, LoadingState } from './StateViews'
import type { AssetDetail } from '../types'

/**
 * The explorer route (/explorer or /explorer/:assetId). The asset ID in the
 * URL is the single source of truth for selection, so a refresh or a direct
 * link reproduces the same view.
 */
export function ExplorerPage() {
  const { assetId } = useParams<{ assetId: string }>()
  const navigate = useNavigate()
  const tree = useAssetTree()

  const [detail, setDetail] = useState<AssetDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState<string | null>(null)

  const selectAsset = useCallback(
    (id: string) => {
      navigate(`/explorer/${encodeURIComponent(id)}`)
    },
    [navigate],
  )

  useEffect(() => {
    if (!assetId) {
      setDetail(null)
      setDetailError(null)
      return
    }
    let cancelled = false
    setDetailLoading(true)
    setDetailError(null)

    Promise.all([getAsset(assetId), tree.revealAsset(assetId)])
      .then(([assetDetail]) => {
        if (cancelled) return
        setDetail(assetDetail)
      })
      .catch((err: Error) => {
        if (cancelled) return
        setDetailError(err.message)
        setDetail(null)
      })
      .finally(() => {
        if (!cancelled) setDetailLoading(false)
      })

    return () => {
      cancelled = true
    }
    // tree.revealAsset intentionally excluded: it is stable across renders
    // and re-running it on every tree state change would cause a fetch loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [assetId])

  return (
    <div className="explorer-layout">
      <aside className="explorer-sidebar">
        <SearchBox onSelect={selectAsset} />
        <div className="tree-scroll">
          <AssetTreeView tree={tree} selectedId={assetId ?? null} onSelect={selectAsset} />
        </div>
      </aside>
      <main className="explorer-main">
        {!assetId && <EmptyState label="Select an asset from the tree or search above to see its details." />}
        {assetId && detailLoading && <LoadingState label="Loading asset details…" />}
        {assetId && detailError && (
          <ErrorState label={`Could not load this asset: ${detailError}`} onRetry={() => selectAsset(assetId)} />
        )}
        {assetId && !detailLoading && !detailError && detail && (
          <AssetDetailsPanel detail={detail} onSelectAncestor={selectAsset} onSelectChild={selectAsset} />
        )}
      </main>
    </div>
  )
}
