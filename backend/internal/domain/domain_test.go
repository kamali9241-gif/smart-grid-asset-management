package domain_test

import (
	"testing"

	"github.com/sivakumarkam/smart-grid/backend/internal/domain"
)

func TestPermittedRelationships(t *testing.T) {
	allowed := map[domain.AssetType]domain.AssetType{
		domain.TypeTransformer:    domain.TypeSubstation,
		domain.TypeLVBoard:        domain.TypeSubstation,
		domain.TypeSwitchboard:    domain.TypeSubstation,
		domain.TypeSwitchboardPnl: domain.TypeSwitchboard,
	}
	for child, parent := range allowed {
		if !domain.IsPermittedRelationship(child, parent) {
			t.Errorf("%s should be allowed under %s", child, parent)
		}
	}

	forbidden := [][2]domain.AssetType{
		{domain.TypeSubstation, domain.TypeSubstation},
		{domain.TypeSwitchboardPnl, domain.TypeSubstation},
		{domain.TypeTransformer, domain.TypeSwitchboard},
		{domain.TypeLVBoard, domain.TypeSwitchboard},
	}
	for _, pair := range forbidden {
		if domain.IsPermittedRelationship(pair[0], pair[1]) {
			t.Errorf("%s must not be allowed under %s", pair[0], pair[1])
		}
	}
}

func TestOnlySubstationIsARoot(t *testing.T) {
	if !domain.IsRootType(domain.TypeSubstation) {
		t.Error("SUBSTATION must be a root type")
	}
	for _, t2 := range []domain.AssetType{domain.TypeTransformer, domain.TypeLVBoard,
		domain.TypeSwitchboard, domain.TypeSwitchboardPnl} {
		if domain.IsRootType(t2) {
			t.Errorf("%s must not be a root type", t2)
		}
	}
}

func TestDescendantCounts(t *testing.T) {
	var c domain.DescendantCounts
	c.Add("TRANSFORMER", 2)
	c.Add("SWITCHBOARD_PANEL", 5)
	c.Add("UNKNOWN", 9)
	if c.Transformers != 2 || c.SwitchboardPanels != 5 {
		t.Fatalf("unexpected buckets: %+v", c)
	}
	if c.Total != 7 {
		t.Fatalf("unknown types must not affect the total, got %d", c.Total)
	}
}

func TestValidEnums(t *testing.T) {
	if !domain.IsValidAssetType("LV_BOARD") || domain.IsValidAssetType("lv_board") {
		t.Error("asset type validation should be case sensitive on canonical values")
	}
	if !domain.IsValidStatus("OUT_OF_SERVICE") || domain.IsValidStatus("RETIRED") {
		t.Error("unexpected status validation result")
	}
}
