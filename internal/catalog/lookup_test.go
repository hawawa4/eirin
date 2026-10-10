package catalog

import (
	"fmt"
	"testing"
)

func TestLookupVariants(t *testing.T) {
	tests := []string{
		"M 86", "M86", "M_86_1584x20sec_foo.png",
		"M60Neighbors", "m47", "NGC 4406",
	}
	for _, q := range tests {
		ra, dec, ok := LookupByName(q)
		fmt.Printf("%-45s → ok=%-5v ra=%.3f dec=%.3f\n", q, ok, ra, dec)
	}
}

func TestLookupByName_Designation(t *testing.T) {
	// Catalog names add a common name after the designation.
	for _, q := range []string{"M 31", "m31", "NGC 7000", "IC 434"} {
		if _, _, ok := LookupByName(q); !ok {
			t.Errorf("LookupByName(%q) found nothing", q)
		}
	}
	// A designation must match whole words, not a prefix of one.
	if _, _, ok := LookupByName("IC 43"); ok {
		t.Error(`"IC 43" should not match "IC 434 Horsehead Region"`)
	}
}
