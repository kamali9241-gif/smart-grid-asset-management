package importer

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// Canonical column names understood by the importer.
const (
	ColAssetID      = "asset_id"
	ColAssetType    = "asset_type"
	ColAssetName    = "asset_name"
	ColParentID     = "parent_asset_id"
	ColStatus       = "operational_status"
	ColCommission   = "commissioned_date"
	ColRatingKVA    = "rating_kva"
	ColVoltageKV    = "voltage_kv"
	ColLocation     = "location"
	ColManufacturer = "manufacturer"
	ColModel        = "model"
	ColSerialNumber = "serial_number"
)

var requiredColumns = []string{ColAssetID, ColAssetType, ColAssetName}

// columnAliases maps normalised header spellings seen in upstream extracts to
// the canonical column names above.
var columnAliases = map[string]string{
	"asset_id":           ColAssetID,
	"id":                 ColAssetID,
	"assetid":            ColAssetID,
	"asset_type":         ColAssetType,
	"type":               ColAssetType,
	"assettype":          ColAssetType,
	"asset_name":         ColAssetName,
	"name":               ColAssetName,
	"assetname":          ColAssetName,
	"parent_asset_id":    ColParentID,
	"parent_id":          ColParentID,
	"parentassetid":      ColParentID,
	"parent":             ColParentID,
	"operational_status": ColStatus,
	"status":             ColStatus,
	"commissioned_date":  ColCommission,
	"commission_date":    ColCommission,
	"commissioned":       ColCommission,
	"rating_kva":         ColRatingKVA,
	"rating":             ColRatingKVA,
	"kva":                ColRatingKVA,
	"voltage_kv":         ColVoltageKV,
	"voltage":            ColVoltageKV,
	"kv":                 ColVoltageKV,
	"location":           ColLocation,
	"site":               ColLocation,
	"manufacturer":       ColManufacturer,
	"model":              ColModel,
	"serial_number":      ColSerialNumber,
	"serialnumber":       ColSerialNumber,
	"serial":             ColSerialNumber,
}

// RawRow is a single CSV data line keyed by canonical column name.
// RowNumber is the physical line number in the file (the header is line 1) so
// rejections can be traced back to the source file.
type RawRow struct {
	RowNumber int
	Values    map[string]string
	Raw       []string
}

func (r RawRow) get(col string) string { return strings.TrimSpace(r.Values[col]) }

// RawLine re-renders the original record for display in the rejection report.
func (r RawRow) RawLine() string { return strings.Join(r.Raw, ",") }

// ParseError describes a problem that prevents the file being read at all.
type ParseError struct{ Message string }

func (e *ParseError) Error() string { return e.Message }

// Parse reads a CSV stream into RawRows. Header order is irrelevant; unknown
// columns are ignored so upstream systems can add fields without breaking us.
func Parse(r io.Reader) ([]RawRow, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1 // ragged rows are reported per-row, not fatally

	header, err := cr.Read()
	if err == io.EOF {
		return nil, &ParseError{Message: "the file is empty"}
	}
	if err != nil {
		return nil, &ParseError{Message: fmt.Sprintf("could not read the CSV header: %v", err)}
	}

	index := map[string]int{}
	for i, h := range header {
		canonical, ok := columnAliases[normaliseHeader(h)]
		if !ok {
			continue
		}
		if _, dup := index[canonical]; dup {
			return nil, &ParseError{Message: fmt.Sprintf("duplicate column %q in the CSV header", canonical)}
		}
		index[canonical] = i
	}
	var missing []string
	for _, c := range requiredColumns {
		if _, ok := index[c]; !ok {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return nil, &ParseError{Message: fmt.Sprintf("missing required column(s): %s", strings.Join(missing, ", "))}
	}

	var rows []RawRow
	line := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			return nil, &ParseError{Message: fmt.Sprintf("malformed CSV at line %d: %v", line, err)}
		}
		if isBlank(rec) {
			continue
		}
		values := make(map[string]string, len(index))
		for col, i := range index {
			if i < len(rec) {
				values[col] = strings.TrimSpace(rec[i])
			}
		}
		rows = append(rows, RawRow{RowNumber: line, Values: values, Raw: rec})
	}
	return rows, nil
}

func normaliseHeader(h string) string {
	h = strings.TrimPrefix(h, "\ufeff")
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.ReplaceAll(h, " ", "_")
	h = strings.ReplaceAll(h, "-", "_")
	return h
}

func isBlank(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}
