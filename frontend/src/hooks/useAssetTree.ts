import { useCallback, useEffect, useRef, useState } from 'react'
import { getAncestors, getChildren, getRoots } from '../api/client'
import type { Asset } from '../types'

interface TreeState {
  roots: Asset[]
  rootsLoading: boolean
  rootsError: string | null
  childrenByParent: Record<string, Asset[] | undefined>
  loadingParents: Set<string>
  expanded: Set<string>
}

/**
 * Owns the lazily-loaded asset tree: root substations plus on-demand children
 * for every expanded node, and the machinery to expand a whole ancestor chain
 * so a searched or deep-linked asset can be revealed without loading everything.
 */
export function useAssetTree() {
  const [state, setState] = useState<TreeState>({
    roots: [],
    rootsLoading: true,
    rootsError: null,
    childrenByParent: {},
    loadingParents: new Set(),
    expanded: new Set(),
  })
  const childrenCache = useRef(new Map<string, Promise<Asset[]>>())

  useEffect(() => {
    let cancelled = false
    getRoots()
      .then((res) => {
        if (cancelled) return
        setState((s) => ({ ...s, roots: res.assets, rootsLoading: false }))
      })
      .catch((err: Error) => {
        if (cancelled) return
        setState((s) => ({ ...s, rootsLoading: false, rootsError: err.message }))
      })
    return () => {
      cancelled = true
    }
  }, [])

  const loadChildren = useCallback((parentId: string): Promise<Asset[]> => {
    const cached = childrenCache.current.get(parentId)
    if (cached) return cached

    setState((s) => ({ ...s, loadingParents: new Set(s.loadingParents).add(parentId) }))
    const promise = getChildren(parentId)
      .then((res) => {
        setState((s) => {
          const loadingParents = new Set(s.loadingParents)
          loadingParents.delete(parentId)
          return {
            ...s,
            childrenByParent: { ...s.childrenByParent, [parentId]: res.assets },
            loadingParents,
          }
        })
        return res.assets
      })
      .catch((err: Error) => {
        setState((s) => {
          const loadingParents = new Set(s.loadingParents)
          loadingParents.delete(parentId)
          return { ...s, loadingParents }
        })
        childrenCache.current.delete(parentId)
        throw err
      })
    childrenCache.current.set(parentId, promise)
    return promise
  }, [])

  const toggleExpanded = useCallback(
    (assetId: string) => {
      setState((s) => {
        const expanded = new Set(s.expanded)
        if (expanded.has(assetId)) {
          expanded.delete(assetId)
        } else {
          expanded.add(assetId)
        }
        return { ...s, expanded }
      })
      if (!state.childrenByParent[assetId]) {
        void loadChildren(assetId)
      }
    },
    [loadChildren, state.childrenByParent],
  )

  // Expands every ancestor of assetId (and loads their children) so the node
  // becomes visible in the tree, e.g. after a search result is selected.
  const revealAsset = useCallback(
    async (assetId: string) => {
      const { assets: ancestors } = await getAncestors(assetId)
      const chain = ancestors.map((a) => a.assetId)
      for (const id of chain) {
        await loadChildren(id)
      }
      setState((s) => ({ ...s, expanded: new Set([...s.expanded, ...chain]) }))
    },
    [loadChildren],
  )

  return {
    roots: state.roots,
    rootsLoading: state.rootsLoading,
    rootsError: state.rootsError,
    childrenByParent: state.childrenByParent,
    loadingParents: state.loadingParents,
    expanded: state.expanded,
    loadChildren,
    toggleExpanded,
    revealAsset,
  }
}

export type AssetTree = ReturnType<typeof useAssetTree>
