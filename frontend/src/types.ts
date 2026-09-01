export type AssetType =
  | 'SUBSTATION'
  | 'TRANSFORMER'
  | 'LV_BOARD'
  | 'SWITCHBOARD'
  | 'SWITCHBOARD_PANEL'

export type OperationalStatus = 'IN_SERVICE' | 'MAINTENANCE' | 'OUT_OF_SERVICE'

export interface Asset {
  assetId: string
  assetType: AssetType
  assetName: string
  parentAssetId: string | null
  operationalStatus: OperationalStatus
  commissionedDate: string | null
  ratingKva: number | null
  voltageKv: number | null
  location: string | null
  manufacturer: string | null
  model: string | null
  serialNumber: string | null
}

export interface ChildGroup {
  assetType: AssetType
  count: number
  assets: Asset[]
}

export interface DescendantCounts {
  SUBSTATION: number
  TRANSFORMER: number
  LV_BOARD: number
  SWITCHBOARD: number
  SWITCHBOARD_PANEL: number
  total: number
}

export interface AssetDetail {
  asset: Asset
  ancestors: Asset[]
  childrenByType: ChildGroup[]
  childCount: number
  descendantCounts: DescendantCounts
}

export interface Rejection {
  rowNumber: number
  assetId?: string
  field?: string
  message: string
  rawRow?: string
}

export type ImportMode = 'all_or_nothing' | 'partial'

export interface ImportReport {
  importId: number
  filename: string
  mode: ImportMode
  totalRows: number
  importedRows: number
  rejectedRows: number
  committed: boolean
  message: string
  rejections: Rejection[]
}

export interface Meta {
  assetTypes: AssetType[]
  operationalStatuses: OperationalStatus[]
  importModes: ImportMode[]
}
