import { useEffect, useRef, useState } from 'react'
import { searchAssets } from '../api/client'
import { useDebouncedValue } from '../hooks/useDebouncedValue'
import type { Asset, AssetType } from '../types'

const TYPE_OPTIONS: { value: AssetType | ''; label: string }[] = [
  { value: '', label: 'All types' },
  { value: 'SUBSTATION', label: 'Substation' },
  { value: 'TRANSFORMER', label: 'Transformer' },
  { value: 'LV_BOARD', label: 'LV Board' },
  { value: 'SWITCHBOARD', label: 'Switchboard' },
  { value: 'SWITCHBOARD_PANEL', label: 'Switchboard Panel' },
]

interface SearchBoxProps {
  onSelect: (assetId: string) => void
}

export function SearchBox({ onSelect }: SearchBoxProps) {
  const [query, setQuery] = useState('')
  const [assetType, setAssetType] = useState<AssetType | ''>('')
  const [results, setResults] = useState<Asset[] | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const debouncedQuery = useDebouncedValue(query.trim(), 250)
  const requestId = useRef(0)

  useEffect(() => {
    if (!debouncedQuery) {
      setResults(null)
      setError(null)
      return
    }
    const id = ++requestId.current
    setLoading(true)
    searchAssets(debouncedQuery, assetType || undefined)
      .then((res) => {
        if (id !== requestId.current) return
        setResults(res.assets)
        setError(null)
      })
      .catch((err: Error) => {
        if (id !== requestId.current) return
        setError(err.message)
        setResults(null)
      })
      .finally(() => {
        if (id === requestId.current) setLoading(false)
      })
  }, [debouncedQuery, assetType])

  return (
    <div className="search-box">
      <div className="search-controls">
        <input
          type="search"
          placeholder="Search by asset ID or name…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          aria-label="Search assets"
        />
        <select
          value={assetType}
          onChange={(e) => setAssetType(e.target.value as AssetType | '')}
          aria-label="Filter by asset type"
        >
          {TYPE_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>
      {loading && <div className="search-status">Searching…</div>}
      {error && <div className="search-status search-error">{error}</div>}
      {results && (
        <ul className="search-results" aria-label="Search results">
          {results.length === 0 && <li className="search-status">No matching assets.</li>}
          {results.map((asset) => (
            <li key={asset.assetId}>
              <button
                type="button"
                onClick={() => {
                  onSelect(asset.assetId)
                  setResults(null)
                  setQuery('')
                }}
              >
                <strong>{asset.assetName}</strong>
                <span className="search-result-meta">
                  {asset.assetType} · {asset.assetId}
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
