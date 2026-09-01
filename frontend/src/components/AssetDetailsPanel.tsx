import type { AssetDetail } from '../types'

const TYPE_LABEL: Record<string, string> = {
  SUBSTATION: 'Substation',
  TRANSFORMER: 'Transformer',
  LV_BOARD: 'LV Board',
  SWITCHBOARD: 'Switchboard',
  SWITCHBOARD_PANEL: 'Switchboard Panel',
}

const STATUS_LABEL: Record<string, string> = {
  IN_SERVICE: 'In service',
  MAINTENANCE: 'Maintenance',
  OUT_OF_SERVICE: 'Out of service',
}

function Field({ label, value }: { label: string; value: string | number | null | undefined }) {
  if (value === null || value === undefined || value === '') return null
  return (
    <div className="detail-field">
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  )
}

interface AssetDetailsPanelProps {
  detail: AssetDetail
  onSelectAncestor: (assetId: string) => void
  onSelectChild: (assetId: string) => void
}

export function AssetDetailsPanel({ detail, onSelectAncestor, onSelectChild }: AssetDetailsPanelProps) {
  const { asset, ancestors, childrenByType, descendantCounts } = detail

  return (
    <div className="details-panel">
      {ancestors.length > 0 && (
        <nav className="breadcrumbs" aria-label="Ancestors">
          {ancestors.map((a) => (
            <span key={a.assetId}>
              <button type="button" onClick={() => onSelectAncestor(a.assetId)}>
                {a.assetName}
              </button>
              <span aria-hidden="true"> / </span>
            </span>
          ))}
        </nav>
      )}

      <header className="details-header">
        <span className="tree-type-badge">{TYPE_LABEL[asset.assetType] ?? asset.assetType}</span>
        <h2>{asset.assetName}</h2>
        <span className={`status-pill status-${asset.operationalStatus.toLowerCase()}`}>
          {STATUS_LABEL[asset.operationalStatus] ?? asset.operationalStatus}
        </span>
      </header>

      <dl className="detail-fields">
        <Field label="Asset ID" value={asset.assetId} />
        <Field label="Parent asset ID" value={asset.parentAssetId} />
        <Field label="Voltage (kV)" value={asset.voltageKv} />
        <Field label="Rating (kVA)" value={asset.ratingKva} />
        <Field label="Manufacturer" value={asset.manufacturer} />
        <Field label="Model" value={asset.model} />
        <Field label="Serial number" value={asset.serialNumber} />
        <Field label="Location" value={asset.location} />
        <Field label="Commissioned" value={asset.commissionedDate} />
      </dl>

      {asset.assetType === 'SUBSTATION' && (
        <section className="descendant-counts" aria-label="Descendant counts">
          <h3>Assets beneath this substation</h3>
          <ul>
            <li>Transformers: {descendantCounts.TRANSFORMER}</li>
            <li>LV boards: {descendantCounts.LV_BOARD}</li>
            <li>Switchboards: {descendantCounts.SWITCHBOARD}</li>
            <li>Switchboard panels: {descendantCounts.SWITCHBOARD_PANEL}</li>
            <li className="total">Total: {descendantCounts.total}</li>
          </ul>
        </section>
      )}

      {childrenByType.length > 0 && (
        <section className="children-groups" aria-label="Immediate children">
          <h3>Immediate children</h3>
          {childrenByType.map((group) => (
            <div key={group.assetType} className="children-group">
              <h4>
                {TYPE_LABEL[group.assetType] ?? group.assetType} ({group.count})
              </h4>
              <ul>
                {group.assets.map((child) => (
                  <li key={child.assetId}>
                    <button type="button" onClick={() => onSelectChild(child.assetId)}>
                      {child.assetName}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </section>
      )}
      {childrenByType.length === 0 && <p className="detail-no-children">This asset has no children.</p>}
    </div>
  )
}
