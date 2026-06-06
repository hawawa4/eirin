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
