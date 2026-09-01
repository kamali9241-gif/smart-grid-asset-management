package importer_test

import (
	"strings"
	"testing"

	"github.com/sivakumarkam/smart-grid/backend/internal/importer"
)

const header = "asset_id,asset_type,asset_name,parent_asset_id,operational_status,commissioned_date,rating_kva,voltage_kv,location\n"

func parse(t *testing.T, body string) []importer.RawRow {
	t.Helper()
	rows, err := importer.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return rows
}

func validate(t *testing.T, body string, existing map[string]importer.ExistingAsset, mode importer.Mode) importer.Result {
	t.Helper()
	if existing == nil {
		existing = map[string]importer.ExistingAsset{}
	}
	return importer.Validate(parse(t, body), existing, mode)
}

func acceptedIDs(res importer.Result) map[string]bool {
	out := map[string]bool{}
	for _, a := range res.Accepted {
		out[a.AssetID] = true
	}
	return out
}

func rejectionFor(res importer.Result, id string) (importer.Rejection, bool) {
	for _, r := range res.Rejections {
		if r.AssetID == id {
			return r, true
		}
	}
	return importer.Rejection{}, false
}

func TestParseRejectsMissingRequiredColumns(t *testing.T) {
	_, err := importer.Parse(strings.NewReader("asset_id,asset_name\nA,Foo\n"))
	if err == nil {
		t.Fatal("expected an error for a file without asset_type")
	}
	if !strings.Contains(err.Error(), "asset_type") {
		t.Fatalf("error should name the missing column, got %q", err)
	}
}

func TestParseAcceptsAliasedHeadersAndIgnoresUnknownColumns(t *testing.T) {
	rows := parse(t, "ID,Type,Name,Parent Id,extra\nSUB-1,SUBSTATION,Aurora,,ignored\n")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	res := importer.Validate(rows, map[string]importer.ExistingAsset{}, importer.ModeAllOrNothing)
	if len(res.Accepted) != 1 {
		t.Fatalf("expected the row to be accepted, rejections: %+v", res.Rejections)
	}
}

func TestParseSkipsBlankLines(t *testing.T) {
	rows := parse(t, header+"SUB-1,SUBSTATION,Aurora,,,,,,\n\n")
	if len(rows) != 1 {
		t.Fatalf("expected blank lines to be skipped, got %d rows", len(rows))
	}
}

// A child may appear before its parent: CSV row order must not affect validity.
func TestValidateIsRowOrderIndependent(t *testing.T) {
	body := header +
		"PNL-1,SWITCHBOARD_PANEL,Panel A1,SWB-1,IN_SERVICE,,,,\n" +
		"SWB-1,SWITCHBOARD,22kV Switchboard A,SUB-1,IN_SERVICE,,,,\n" +
		"SUB-1,SUBSTATION,Aurora Substation,,IN_SERVICE,,,,\n"

	res := validate(t, body, nil, importer.ModeAllOrNothing)
	if len(res.Rejections) != 0 {
		t.Fatalf("expected no rejections, got %+v", res.Rejections)
	}
	if len(res.Accepted) != 3 {
		t.Fatalf("expected 3 accepted rows, got %d", len(res.Accepted))
	}
}

func TestValidateFieldLevelRules(t *testing.T) {
	cases := []struct {
		name    string
		row     string
		id      string
		wantMsg string
	}{
		{"missing asset_id", ",SUBSTATION,Aurora,,,,,,", "", "asset_id is required"},
		{"missing asset_name", "SUB-1,SUBSTATION,,,,,,,", "SUB-1", "asset_name is required"},
		{"unknown type", "X-1,GENERATOR,Gen,,,,,,", "X-1", "unsupported asset_type"},
		{"unknown status", "SUB-1,SUBSTATION,Aurora,,RETIRED,,,,", "SUB-1", "unsupported operational_status"},
		{"bad date", "SUB-1,SUBSTATION,Aurora,,IN_SERVICE,not-a-date,,,", "SUB-1", "not a valid date"},
		{"bad number", "SUB-1,SUBSTATION,Aurora,,IN_SERVICE,,abc,,", "SUB-1", "not a valid number"},
		{"negative rating", "SUB-1,SUBSTATION,Aurora,,IN_SERVICE,,-5,,", "SUB-1", "rating_kva cannot be negative"},
		{"substation with parent", "SUB-1,SUBSTATION,Aurora,SUB-2,,,,,", "SUB-1", "cannot have a parent_asset_id"},
		{"child without parent", "TRF-1,TRANSFORMER,T1,,,,,,", "TRF-1", "parent_asset_id is required"},
		{"self parent", "TRF-1,TRANSFORMER,T1,TRF-1,,,,,", "TRF-1", "cannot be its own parent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := validate(t, header+tc.row+"\n", nil, importer.ModeAllOrNothing)
			if len(res.Accepted) != 0 {
				t.Fatalf("expected the row to be rejected")
			}
			var found bool
			for _, r := range res.Rejections {
				if strings.Contains(r.Message, tc.wantMsg) {
					found = true
					if r.RowNumber != 2 {
						t.Errorf("expected row number 2, got %d", r.RowNumber)
					}
					if tc.id != "" && r.AssetID != tc.id {
						t.Errorf("expected asset id %q, got %q", tc.id, r.AssetID)
					}
					if r.RawRow == "" {
						t.Error("expected the original CSV row to be reported")
					}
				}
			}
			if !found {
				t.Fatalf("expected a rejection containing %q, got %+v", tc.wantMsg, res.Rejections)
			}
		})
	}
}

func TestValidateStatusDefaultsToInService(t *testing.T) {
	res := validate(t, header+"SUB-1,SUBSTATION,Aurora,,,,,,\n", nil, importer.ModeAllOrNothing)
	if len(res.Accepted) != 1 {
		t.Fatalf("expected 1 accepted row, got %+v", res.Rejections)
	}
	if got := res.Accepted[0].OperationalStatus; got != "IN_SERVICE" {
		t.Fatalf("expected IN_SERVICE default, got %q", got)
	}
}

func TestValidateNormalisesDates(t *testing.T) {
	res := validate(t, header+"SUB-1,SUBSTATION,Aurora,,IN_SERVICE,12/03/2019,,,\n", nil, importer.ModeAllOrNothing)
	if len(res.Accepted) != 1 {
		t.Fatalf("expected acceptance, got %+v", res.Rejections)
	}
	if got := *res.Accepted[0].CommissionedDate; got != "2019-03-12" {
		t.Fatalf("expected 2019-03-12, got %s", got)
	}
}

func TestValidateRejectsDuplicateIDsInFile(t *testing.T) {
	body := header +
		"SUB-1,SUBSTATION,Aurora,,,,,,\n" +
		"SUB-1,SUBSTATION,Aurora Copy,,,,,,\n"

	res := validate(t, body, nil, importer.ModeAllOrNothing)
	if len(res.Accepted) != 0 {
		t.Fatal("both occurrences of a duplicated id must be rejected")
	}
	if len(res.Rejections) != 2 {
		t.Fatalf("expected 2 rejections, got %+v", res.Rejections)
	}
}

func TestValidateRejectsIDsAlreadyInDatabase(t *testing.T) {
	existing := map[string]importer.ExistingAsset{
		"SUB-1": {AssetID: "SUB-1", AssetType: "SUBSTATION"},
	}
	res := validate(t, header+"SUB-1,SUBSTATION,Aurora,,,,,,\n", existing, importer.ModeAllOrNothing)
	r, ok := rejectionFor(res, "SUB-1")
	if !ok || !strings.Contains(r.Message, "already exists in the database") {
		t.Fatalf("expected a duplicate-in-database rejection, got %+v", res.Rejections)
	}
}

func TestValidateResolvesParentFromDatabase(t *testing.T) {
	existing := map[string]importer.ExistingAsset{
		"SUB-1": {AssetID: "SUB-1", AssetType: "SUBSTATION"},
	}
	res := validate(t, header+"TRF-9,TRANSFORMER,T9,SUB-1,,,,,\n", existing, importer.ModeAllOrNothing)
	if len(res.Accepted) != 1 {
		t.Fatalf("expected the transformer to attach to the stored substation, got %+v", res.Rejections)
	}
}

func TestValidateRejectsMissingParent(t *testing.T) {
	res := validate(t, header+"TRF-1,TRANSFORMER,T1,SUB-404,,,,,\n", nil, importer.ModeAllOrNothing)
	r, ok := rejectionFor(res, "TRF-1")
	if !ok || !strings.Contains(r.Message, "not found in the file or in the database") {
		t.Fatalf("expected a missing-parent rejection, got %+v", res.Rejections)
	}
}

func TestValidateRejectsForbiddenParentType(t *testing.T) {
	body := header +
		"SUB-1,SUBSTATION,Aurora,,,,,,\n" +
		"PNL-1,SWITCHBOARD_PANEL,Panel A1,SUB-1,,,,,\n"

	res := validate(t, body, nil, importer.ModeAllOrNothing)
	r, ok := rejectionFor(res, "PNL-1")
	if !ok || !strings.Contains(r.Message, "may only be a child of SWITCHBOARD") {
		t.Fatalf("expected a permitted-hierarchy rejection, got %+v", res.Rejections)
	}
	if acceptedIDs(res)["PNL-1"] {
		t.Fatal("panel must not be accepted under a substation")
	}
}

func TestValidateDetectsCycles(t *testing.T) {
	// TRF-1 -> SWB-1 -> TRF-1 would never reach a substation root.
	body := header +
		"TRF-1,TRANSFORMER,T1,SWB-1,,,,,\n" +
		"SWB-1,SWITCHBOARD,SB1,TRF-1,,,,,\n"

	res := validate(t, body, nil, importer.ModeAllOrNothing)
	if len(res.Accepted) != 0 {
		t.Fatalf("cyclic rows must not be accepted, got %+v", res.Accepted)
	}
	if len(res.Rejections) == 0 {
		t.Fatal("expected rejections for the cycle")
	}
}

func TestValidateRejectsSubtreeOfARejectedAncestor(t *testing.T) {
	// SWB-1 references a substation that does not exist, so its panel cannot
	// resolve to a root either.
	body := header +
		"SWB-1,SWITCHBOARD,SB1,SUB-404,,,,,\n" +
		"PNL-1,SWITCHBOARD_PANEL,Panel A1,SWB-1,,,,,\n"

	res := validate(t, body, nil, importer.ModePartial)
	if len(res.Accepted) != 0 {
		t.Fatalf("expected the whole broken branch to be rejected, got %+v", res.Accepted)
	}
	r, ok := rejectionFor(res, "PNL-1")
	if !ok || !strings.Contains(r.Message, "rejected") {
		t.Fatalf("expected a cascade rejection for PNL-1, got %+v", res.Rejections)
	}
}

func TestValidatePartialModeKeepsHealthyBranches(t *testing.T) {
	body := header +
		"SUB-1,SUBSTATION,Aurora,,,,,,\n" +
		"TRF-1,TRANSFORMER,T1,SUB-1,,,,,\n" +
		"TRF-2,TRANSFORMER,T2,SUB-404,,,,,\n"

	res := validate(t, body, nil, importer.ModePartial)
	accepted := acceptedIDs(res)
	if !accepted["SUB-1"] || !accepted["TRF-1"] {
		t.Fatalf("valid branch should survive, accepted=%v", accepted)
	}
	if accepted["TRF-2"] {
		t.Fatal("the orphan transformer must be rejected")
	}
	if res.TotalRows != 3 || len(res.Rejections) != 1 {
		t.Fatalf("expected 3 total rows and 1 rejection, got %d/%d", res.TotalRows, len(res.Rejections))
	}
}

func TestRejectionsAreOrderedByRowNumber(t *testing.T) {
	body := header +
		"SUB-1,SUBSTATION,Aurora,,,,,,\n" +
		"BAD-2,TRANSFORMER,T,SUB-404,,,,,\n" +
		"BAD-3,GENERATOR,G,,,,,,\n"

	res := validate(t, body, nil, importer.ModePartial)
	if len(res.Rejections) < 2 {
		t.Fatalf("expected at least 2 rejections, got %+v", res.Rejections)
	}
	for i := 1; i < len(res.Rejections); i++ {
		if res.Rejections[i-1].RowNumber > res.Rejections[i].RowNumber {
			t.Fatalf("rejections are not sorted by row number: %+v", res.Rejections)
		}
	}
}

func TestReferencedIDsCollectsAssetAndParentIDs(t *testing.T) {
	rows := parse(t, header+"TRF-1,TRANSFORMER,T1,SUB-1,,,,,\n")
	ids := importer.ReferencedIDs(rows)
	if len(ids) != 2 {
		t.Fatalf("expected 2 referenced ids, got %v", ids)
	}
}

func TestParseMode(t *testing.T) {
	if m, err := importer.ParseMode(""); err != nil || m != importer.ModeAllOrNothing {
		t.Fatalf("empty mode should default to all_or_nothing, got %v %v", m, err)
	}
	if m, err := importer.ParseMode("partial"); err != nil || m != importer.ModePartial {
		t.Fatalf("expected partial mode, got %v %v", m, err)
	}
	if _, err := importer.ParseMode("whatever"); err == nil {
		t.Fatal("expected an error for an unsupported mode")
	}
}
