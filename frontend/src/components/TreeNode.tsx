import type { Asset } from '../types'
import type { AssetTree } from '../hooks/useAssetTree'
import { LoadingState } from './StateViews'

const TYPE_LABEL: Record<string, string> = {
  SUBSTATION: 'Substation',
  TRANSFORMER: 'Transformer',
  LV_BOARD: 'LV Board',
  SWITCHBOARD: 'Switchboard',
  SWITCHBOARD_PANEL: 'Panel',
}

interface TreeNodeProps {
  asset: Asset
  tree: AssetTree
  selectedId: string | null
  onSelect: (assetId: string) => void
  depth: number
}

// A node may only ever have children if its type appears as a permitted
// parent; leaf types (transformers, LV boards, panels) never show a toggle.
const CAN_HAVE_CHILDREN = new Set(['SUBSTATION', 'SWITCHBOARD'])

export function TreeNode({ asset, tree, selectedId, onSelect, depth }: TreeNodeProps) {
  const isExpandable = CAN_HAVE_CHILDREN.has(asset.assetType)
  const isExpanded = tree.expanded.has(asset.assetId)
  const isLoading = tree.loadingParents.has(asset.assetId)
  const children = tree.childrenByParent[asset.assetId]
  const isSelected = asset.assetId === selectedId

  return (
    <li role="treeitem" aria-expanded={isExpandable ? isExpanded : undefined} aria-selected={isSelected}>
      <div className="tree-row" style={{ paddingLeft: depth * 16 }}>
        {isExpandable ? (
          <button
            type="button"
            className="tree-toggle"
            aria-label={isExpanded ? `Collapse ${asset.assetName}` : `Expand ${asset.assetName}`}
            onClick={() => tree.toggleExpanded(asset.assetId)}
          >
            {isExpanded ? '▾' : '▸'}
          </button>
        ) : (
          <span className="tree-toggle-spacer" />
        )}
        <button
          type="button"
          className={`tree-label${isSelected ? ' tree-label-selected' : ''}`}
          onClick={() => onSelect(asset.assetId)}
        >
          <span className="tree-type-badge">{TYPE_LABEL[asset.assetType] ?? asset.assetType}</span>
          <span>{asset.assetName}</span>
        </button>
      </div>
      {isExpandable && isExpanded && (
        <ul role="group">
          {isLoading && !children && (
            <li className="tree-inline-state">
              <LoadingState label="Loading children…" />
            </li>
          )}
          {children?.length === 0 && <li className="tree-inline-state tree-empty">No children</li>}
          {children?.map((child) => (
            <TreeNode
              key={child.assetId}
              asset={child}
              tree={tree}
              selectedId={selectedId}
              onSelect={onSelect}
              depth={depth + 1}
            />
          ))}
        </ul>
      )}
    </li>
  )
}
