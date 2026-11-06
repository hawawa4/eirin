package gallery

import (
	"strings"
	"testing"
)

func TestPublicPathHidesTheRealOne(t *testing.T) {
	real := "/library/Andromeda/Stacked_30_M 31_10.0s_20240101-221530.JPG"
	p := PublicPath(real)
	if !strings.HasPrefix(p, Prefix) || !strings.HasSuffix(p, ".jpg") {
		t.Errorf("PublicPath = %q, want %s<hash>.jpg", p, Prefix)
	}
	for _, leak := range []string{"library", "Andromeda", "Stacked", "M 31"} {
		if strings.Contains(p, leak) {
			t.Errorf("PublicPath %q leaks %q", p, leak)
		}
	}
	if PublicPath(real) != p {
		t.Error("PublicPath must be stable")
	}
	if PublicPath(real+"x") == p {
		t.Error("different files must get different paths")
	}
}

func TestIndexResolvesOnlyPublicPaths(t *testing.T) {
	real := "/library/a.fit"
	ix := NewIndex([]string{real, "/library/b.png"})
	if got, ok := ix.Real(PublicPath(real)); !ok || got != real {
		t.Errorf("Real(public) = %q, %v", got, ok)
	}
	if _, ok := ix.Real(real); ok {
		t.Error("a real path must not resolve")
	}
	if _, ok := ix.Real("/g/ffffffffffffffff.fit"); ok {
		t.Error("an unknown id must not resolve")
	}
	if ix.Len() != 2 {
		t.Errorf("Len = %d, want 2", ix.Len())
	}
	var nilIx *Index
	if _, ok := nilIx.Real(PublicPath(real)); ok || nilIx.Len() != 0 {
		t.Error("a nil index must resolve nothing")
	}
}
