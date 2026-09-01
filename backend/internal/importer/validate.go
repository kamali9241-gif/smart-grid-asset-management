package importer

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sivakumarkam/smart-grid/backend/internal/domain"
)

// Mode selects the transactional behaviour of an import.
type Mode string

const (
	// ModeAllOrNothing commits only when every row is valid (default).
	ModeAllOrNothing Mode = "all_or_nothing"
	// ModePartial commits the valid rows and reports the rest.
	ModePartial Mode = "partial"
)

func ParseMode(v string) (Mode, error) {
	switch Mode(strings.TrimSpace(strings.ToLower(v))) {
	case "", ModeAllOrNothing:
		return ModeAllOrNothing, nil
	case ModePartial:
		return ModePartial, nil
	default:
		return "", fmt.Errorf("unsupported import mode %q (expected all_or_nothing or partial)", v)
	}
}

// Rejection explains why a single CSV row was not accepted.
type Rejection struct {
	RowNumber int    `json:"rowNumber"`
	AssetID   string `json:"assetId,omitempty"`
	Field     string `json:"field,omitempty"`
	Message   string `json:"message"`
	RawRow    string `json:"rawRow,omitempty"`
}

// Result is the outcome of validating a whole file.
type Result struct {
	Mode       Mode
	TotalRows  int
	Accepted   []domain.Asset
	Rejections []Rejection
}

// ExistingAsset is the slice of a persisted asset the validator needs.
type ExistingAsset struct {
	AssetID   string
	AssetType string
}

// dateLayouts covers ISO-8601 plus the day/month upstream extract formats
// seen in practice, including non-zero-padded day/month and 2-digit years
// (e.g. "15/2/09").
var dateLayouts = []string{
	"2006-01-02", "2006/01/02",
	"02/01/2006", "02-01-2006",
	"2/1/2006", "2/1/06",
	time.RFC3339,
}

type candidate struct {
	row    RawRow
	asset  domain.Asset
	parent string
}

// Validate applies every documented rule to the parsed rows.
//
// It is deliberately pure: `existing` is the set of assets already in
// PostgreSQL that the rows refer to (by id or parent id). Because the database
// enforces the same hierarchy constraints, any parent that already exists there
// is known to be acyclic and rooted at a substation, so the graph walk can stop
// as soon as it reaches one.
func Validate(rows []RawRow, existing map[string]ExistingAsset, mode Mode) Result {
	res := Result{Mode: mode, TotalRows: len(rows)}
	reject := func(r RawRow, id, field, msg string) {
		res.Rejections = append(res.Rejections, Rejection{
			RowNumber: r.RowNumber, AssetID: id, Field: field, Message: msg, RawRow: r.RawLine(),
		})
	}

	// Stage 1 - field level validation.
	candidates := make([]*candidate, 0, len(rows))
	occurrences := map[string]int{}
	for _, row := range rows {
		id := row.get(ColAssetID)
		if id != "" {
			occurrences[id]++
		}
		c, errs := validateFields(row)
		if len(errs) > 0 {
			for _, e := range errs {
				reject(row, id, e.field, e.message)
			}
			continue
		}
		candidates = append(candidates, c)
	}

	// Stage 2 - uniqueness inside the file and against the database.
	// Every occurrence of a duplicated id is rejected: picking one silently
	// would hide a genuine upstream data-quality problem.
	byID := map[string]*candidate{}
	surviving := make([]*candidate, 0, len(candidates))
	for _, c := range candidates {
		id := c.asset.AssetID
		if occurrences[id] > 1 {
			reject(c.row, id, ColAssetID,
				fmt.Sprintf("duplicate asset_id %q appears %d times in the file", id, occurrences[id]))
			continue
		}
		if _, ok := existing[id]; ok {
			reject(c.row, id, ColAssetID, fmt.Sprintf("asset_id %q already exists in the database", id))
			continue
		}
		byID[id] = c
		surviving = append(surviving, c)
	}

	// Stage 3 - hierarchy resolution (parent existence, permitted pairing,
	// cycle detection and reachability of a substation root).
	state := map[string]int{} // 0 unvisited, 1 in progress, 2 resolved, 3 rejected
	reasons := map[string]Rejection{}
	var resolve func(id string, path []string) bool
	resolve = func(id string, path []string) bool {
		switch state[id] {
		case 2:
			return true
		case 3:
			return false
		case 1:
			// Close the cycle and reject every node taking part in it.
			start := 0
			for i, p := range path {
				if p == id {
					start = i
					break
				}
			}
			loop := append(append([]string{}, path[start:]...), id)
			for _, member := range loop[:len(loop)-1] {
				state[member] = 3
				c := byID[member]
				reasons[member] = Rejection{
					RowNumber: c.row.RowNumber, AssetID: member, Field: ColParentID,
					Message: "circular parent relationship detected: " + strings.Join(loop, " -> "),
					RawRow:  c.row.RawLine(),
				}
			}
			return false
		}

		c, inFile := byID[id]
		if !inFile {
			// Resolved against the database, which is acyclic by construction.
			state[id] = 2
			return true
		}
		state[id] = 1
		ok := func() bool {
			if domain.IsRootType(domain.AssetType(c.asset.AssetType)) {
				return true
			}
			parentID := c.parent
			var parentType string
			if p, ok := byID[parentID]; ok {
				parentType = p.asset.AssetType
			} else if e, ok := existing[parentID]; ok {
				parentType = e.AssetType
			} else {
				msg := fmt.Sprintf("parent %q was not found in the file or in the database", parentID)
				if occurrences[parentID] > 0 {
					msg = fmt.Sprintf("parent %q is present in the file but was itself rejected", parentID)
				}
				reasons[id] = Rejection{
					RowNumber: c.row.RowNumber, AssetID: id, Field: ColParentID,
					Message: msg,
					RawRow:  c.row.RawLine(),
				}
				return false
			}
			if !domain.IsPermittedRelationship(domain.AssetType(c.asset.AssetType), domain.AssetType(parentType)) {
				want, _ := domain.PermittedParentOf(domain.AssetType(c.asset.AssetType))
				reasons[id] = Rejection{
					RowNumber: c.row.RowNumber, AssetID: id, Field: ColParentID,
					Message: fmt.Sprintf("%s may only be a child of %s, but parent %q is a %s",
						c.asset.AssetType, want, parentID, parentType),
					RawRow: c.row.RawLine(),
				}
				return false
			}
			if !resolve(parentID, append(path, id)) {
				if _, already := reasons[id]; !already {
					reasons[id] = Rejection{
						RowNumber: c.row.RowNumber, AssetID: id, Field: ColParentID,
						Message: fmt.Sprintf("parent %q was rejected, so this asset cannot resolve to a substation root", parentID),
						RawRow:  c.row.RawLine(),
					}
				}
				return false
			}
			return true
		}()
		if state[id] == 3 { // marked by cycle detection while unwinding
			return false
		}
		if ok {
			state[id] = 2
		} else {
			state[id] = 3
		}
		return ok
	}

	for _, c := range surviving {
		resolve(c.asset.AssetID, nil)
	}

	for _, c := range surviving {
		id := c.asset.AssetID
		if state[id] == 2 {
			res.Accepted = append(res.Accepted, c.asset)
			continue
		}
		if r, ok := reasons[id]; ok {
			res.Rejections = append(res.Rejections, r)
		} else {
			reject(c.row, id, ColParentID, "asset does not resolve to a substation root")
		}
	}

	sort.SliceStable(res.Rejections, func(i, j int) bool {
		return res.Rejections[i].RowNumber < res.Rejections[j].RowNumber
	})
	return res
}

type fieldError struct{ field, message string }

func validateFields(row RawRow) (*candidate, []fieldError) {
	var errs []fieldError
	fail := func(field, msg string) { errs = append(errs, fieldError{field, msg}) }

	id := row.get(ColAssetID)
	if id == "" {
		fail(ColAssetID, "asset_id is required")
	}
	name := row.get(ColAssetName)
	if name == "" {
		fail(ColAssetName, "asset_name is required")
	}

	rawType := strings.ToUpper(row.get(ColAssetType))
	switch {
	case rawType == "":
		fail(ColAssetType, "asset_type is required")
	case !domain.IsValidAssetType(rawType):
		fail(ColAssetType, fmt.Sprintf("unsupported asset_type %q (expected one of %s)",
			row.get(ColAssetType), strings.Join(domain.AssetTypes(), ", ")))
	}

	status := strings.ToUpper(row.get(ColStatus))
	if status == "" {
		status = string(domain.StatusInService)
	} else if !domain.IsValidStatus(status) {
		fail(ColStatus, fmt.Sprintf("unsupported operational_status %q (expected one of %s)",
			row.get(ColStatus), strings.Join(domain.Statuses(), ", ")))
	}

	parent := row.get(ColParentID)
	if rawType != "" && domain.IsValidAssetType(rawType) {
		isRoot := domain.IsRootType(domain.AssetType(rawType))
		switch {
		case isRoot && parent != "":
			fail(ColParentID, "a SUBSTATION is a tree root and cannot have a parent_asset_id")
		case !isRoot && parent == "":
			fail(ColParentID, fmt.Sprintf("parent_asset_id is required for %s", rawType))
		}
	}
	if parent != "" && id != "" && parent == id {
		fail(ColParentID, "an asset cannot be its own parent")
	}

	commissioned, err := parseDate(row.get(ColCommission))
	if err != nil {
		fail(ColCommission, err.Error())
	}
	rating, err := parseNumber(row.get(ColRatingKVA), ColRatingKVA)
	if err != nil {
		fail(ColRatingKVA, err.Error())
	}
	if rating != nil && *rating < 0 {
		fail(ColRatingKVA, "rating_kva cannot be negative")
	}
	voltage, err := parseNumber(row.get(ColVoltageKV), ColVoltageKV)
	if err != nil {
		fail(ColVoltageKV, err.Error())
	}
	if voltage != nil && *voltage < 0 {
		fail(ColVoltageKV, "voltage_kv cannot be negative")
	}

	if len(errs) > 0 {
		return nil, errs
	}

	asset := domain.Asset{
		AssetID:           id,
		AssetType:         rawType,
		AssetName:         name,
		OperationalStatus: status,
		CommissionedDate:  commissioned,
		RatingKVA:         rating,
		VoltageKV:         voltage,
	}
	if parent != "" {
		p := parent
		asset.ParentAssetID = &p
	}
	if loc := row.get(ColLocation); loc != "" {
		asset.Location = &loc
	}
	if v := row.get(ColManufacturer); v != "" {
		asset.Manufacturer = &v
	}
	if v := row.get(ColModel); v != "" {
		asset.Model = &v
	}
	if v := row.get(ColSerialNumber); v != "" {
		asset.SerialNumber = &v
	}
	return &candidate{row: row, asset: asset, parent: parent}, nil
}

func parseDate(v string) (*string, error) {
	if v == "" {
		return nil, nil
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, v); err == nil {
			s := t.Format("2006-01-02")
			return &s, nil
		}
	}
	return nil, fmt.Errorf("%q is not a valid date (expected YYYY-MM-DD)", v)
}

func parseNumber(v, field string) (*float64, error) {
	if v == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", ""), 64)
	if err != nil {
		return nil, fmt.Errorf("%q is not a valid number for %s", v, field)
	}
	return &f, nil
}

// ReferencedIDs returns every asset id and parent id mentioned by the rows so
// the repository can fetch the relevant existing assets in a single query.
func ReferencedIDs(rows []RawRow) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(rows)*2)
	for _, r := range rows {
		for _, col := range []string{ColAssetID, ColParentID} {
			if v := r.get(col); v != "" {
				if _, ok := seen[v]; !ok {
					seen[v] = struct{}{}
					out = append(out, v)
				}
			}
		}
	}
	return out
}
