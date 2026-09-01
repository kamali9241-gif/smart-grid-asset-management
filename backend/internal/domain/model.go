package domain

// Asset is the canonical representation of a stored asset record.
// Dates are carried as ISO-8601 strings (YYYY-MM-DD) so the API contract does
// not leak timezone semantics onto a calendar date.
type Asset struct {
	AssetID           string   `json:"assetId"`
	AssetType         string   `json:"assetType"`
	AssetName         string   `json:"assetName"`
	ParentAssetID     *string  `json:"parentAssetId"`
	OperationalStatus string   `json:"operationalStatus"`
	CommissionedDate  *string  `json:"commissionedDate"`
	RatingKVA         *float64 `json:"ratingKva"`
	VoltageKV         *float64 `json:"voltageKv"`
	Location          *string  `json:"location"`
	Manufacturer      *string  `json:"manufacturer"`
	Model             *string  `json:"model"`
	SerialNumber      *string  `json:"serialNumber"`
}

// ChildGroup buckets immediate children by asset type for the details panel.
type ChildGroup struct {
	AssetType string  `json:"assetType"`
	Count     int     `json:"count"`
	Assets    []Asset `json:"assets"`
}

// DescendantCounts is the roll-up shown for a selected asset.
type DescendantCounts struct {
	Transformers      int `json:"TRANSFORMER"`
	LVBoards          int `json:"LV_BOARD"`
	Switchboards      int `json:"SWITCHBOARD"`
	SwitchboardPanels int `json:"SWITCHBOARD_PANEL"`
	Substations       int `json:"SUBSTATION"`
	Total             int `json:"total"`
}

// Add increments the bucket matching t. Unknown types are ignored.
func (d *DescendantCounts) Add(t string, n int) {
	switch AssetType(t) {
	case TypeTransformer:
		d.Transformers += n
	case TypeLVBoard:
		d.LVBoards += n
	case TypeSwitchboard:
		d.Switchboards += n
	case TypeSwitchboardPnl:
		d.SwitchboardPanels += n
	case TypeSubstation:
		d.Substations += n
	default:
		return
	}
	d.Total += n
}
