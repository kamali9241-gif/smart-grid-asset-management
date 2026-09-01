// Package domain holds the core asset vocabulary and hierarchy rules.
// It has no dependency on the database or transport layers so the rules can be
// unit tested in isolation.
package domain

import "sort"

type AssetType string

const (
	TypeSubstation     AssetType = "SUBSTATION"
	TypeTransformer    AssetType = "TRANSFORMER"
	TypeLVBoard        AssetType = "LV_BOARD"
	TypeSwitchboard    AssetType = "SWITCHBOARD"
	TypeSwitchboardPnl AssetType = "SWITCHBOARD_PANEL"
)

type OperationalStatus string

const (
	StatusInService    OperationalStatus = "IN_SERVICE"
	StatusMaintenance  OperationalStatus = "MAINTENANCE"
	StatusOutOfService OperationalStatus = "OUT_OF_SERVICE"
)

// permittedParent maps an asset type to the single asset type it may hang off.
// The root type (SUBSTATION) is deliberately absent: it may not have a parent.
var permittedParent = map[AssetType]AssetType{
	TypeTransformer:    TypeSubstation,
	TypeLVBoard:        TypeSubstation,
	TypeSwitchboard:    TypeSubstation,
	TypeSwitchboardPnl: TypeSwitchboard,
}

var allTypes = []AssetType{
	TypeSubstation, TypeTransformer, TypeLVBoard, TypeSwitchboard, TypeSwitchboardPnl,
}

var allStatuses = []OperationalStatus{
	StatusInService, StatusMaintenance, StatusOutOfService,
}

func IsValidAssetType(v string) bool {
	for _, t := range allTypes {
		if string(t) == v {
			return true
		}
	}
	return false
}

func IsValidStatus(v string) bool {
	for _, s := range allStatuses {
		if string(s) == v {
			return true
		}
	}
	return false
}

// IsRootType reports whether the type sits at the top of an asset tree.
func IsRootType(t AssetType) bool { return t == TypeSubstation }

// PermittedParentOf returns the required parent type and whether the child type
// is allowed to have a parent at all.
func PermittedParentOf(child AssetType) (AssetType, bool) {
	p, ok := permittedParent[child]
	return p, ok
}

// IsPermittedRelationship reports whether child may be attached to parent.
func IsPermittedRelationship(child, parent AssetType) bool {
	want, ok := permittedParent[child]
	return ok && want == parent
}

func AssetTypes() []string {
	out := make([]string, 0, len(allTypes))
	for _, t := range allTypes {
		out = append(out, string(t))
	}
	return out
}

func Statuses() []string {
	out := make([]string, 0, len(allStatuses))
	for _, s := range allStatuses {
		out = append(out, string(s))
	}
	sort.Strings(out)
	return out
}
