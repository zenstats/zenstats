package types

import "testing"

func TestParseRawFiltersJSONParsesListFormat(t *testing.T) {
	filters, err := ParseRawFiltersJSON(`[["is","visit:browser",["Chrome"]]]`)
	if err != nil {
		t.Fatalf("ParseRawFiltersJSON returned error: %v", err)
	}
	if len(filters) != 1 {
		t.Fatalf("len(filters) = %d, want 1", len(filters))
	}
	if filters[0].Operator != "is" || filters[0].Dimension != "visit:browser" || filters[0].Values[0] != "Chrome" {
		t.Fatalf("unexpected filter: %#v", filters[0])
	}
}

func TestParseRawFiltersJSONParsesCompoundFormat(t *testing.T) {
	filters, err := ParseRawFiltersJSON(`["and",[["is","visit:browser",["Chrome"]],["is","visit:country",["CN"]]]]`)
	if err != nil {
		t.Fatalf("ParseRawFiltersJSON returned error: %v", err)
	}
	if len(filters) != 1 || filters[0].Operator != "and" {
		t.Fatalf("unexpected compound filters: %#v", filters)
	}
	if len(filters[0].SubFilters) != 2 {
		t.Fatalf("len(subfilters) = %d, want 2", len(filters[0].SubFilters))
	}
}

func TestParseRawFiltersJSONEmpty(t *testing.T) {
	filters, err := ParseRawFiltersJSON("")
	if err != nil {
		t.Fatalf("empty filters returned error: %v", err)
	}
	if filters != nil {
		t.Fatalf("empty filters = %#v, want nil", filters)
	}
}
