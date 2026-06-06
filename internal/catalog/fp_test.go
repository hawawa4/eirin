package catalog

import (
	"fmt"
	"testing"
)

func TestLookupFalsePositives(t *testing.T) {
	tests := []string{"M100_processed.png", "M10_stacked.png", "M1_data.fit"}
	for _, q := range tests {
		ra, dec, ok := LookupByName(q)
		if ok {
			fmt.Printf("%-35s → M? ra=%.3f dec=%.3f\n", q, ra, dec)
		}
	}
}
