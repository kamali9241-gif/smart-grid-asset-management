import { useAssetTree } from '../hooks/useAssetTree'
import { TreeNode } from './TreeNode'
import { EmptyState, ErrorState, LoadingState } from './StateViews'

interface AssetTreeViewProps {
  tree: ReturnType<typeof useAssetTree>
  selectedId: string | null
  onSelect: (assetId: string) => void
}

export function AssetTreeView({ tree, selectedId, onSelect }: AssetTreeViewProps) {
  if (tree.rootsLoading) return <LoadingState label="Loading substations…" />
  if (tree.rootsError) return <ErrorState label={`Could not load the asset tree: ${tree.rootsError}`} />
  if (tree.roots.length === 0) {
    return <EmptyState label="No assets yet. Upload a CSV to get started." />
  }
  return (
    <ul role="tree" className="asset-tree" aria-label="Asset hierarchy">
      {tree.roots.map((root) => (
        <TreeNode key={root.assetId} asset={root} tree={tree} selectedId={selectedId} onSelect={onSelect} depth={0} />
      ))}
    </ul>
  )
}
