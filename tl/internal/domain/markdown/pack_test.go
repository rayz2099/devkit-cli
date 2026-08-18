package markdown

import (
	"fmt"
	"strings"
	"testing"
)

func TestFastPackBudgetScalesWithDocumentSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		size int
		want int
	}{
		{name: "tiny", size: 100, want: 16 << 10},
		{name: "small", size: 2 << 10, want: 16 << 10},
		{name: "medium", size: 8 << 10, want: 4 << 10},
		{name: "large", size: 8<<10 + 1, want: 8 << 10},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := FastPackBudget(tc.size); got != tc.want {
				t.Fatalf("FastPackBudget(%d) = %d, want %d", tc.size, got, tc.want)
			}
		})
	}
}

func TestPackUnitIndexesMergesUntilBudget(t *testing.T) {
	t.Parallel()

	units := []TextUnit{
		{Text: "aaa"},
		{Text: "bbb"},
		{Text: strings.Repeat("c", 8)},
	}
	got := PackUnitIndexes(units, 6)
	want := [][]int{{0, 1}, {2}}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Fatalf("groups = %#v, want %#v", got, want)
	}
}

func TestEncodeDecodePackRoundTripPreservesTrailingNewline(t *testing.T) {
	t.Parallel()

	originals := []string{"# Hello\n", "More text"}
	packed := EncodePack(originals)
	if !strings.Contains(packed, "<<<TL 0>>>") || !strings.Contains(packed, "<<<TL 1>>>") {
		t.Fatalf("packed = %q, want markers", packed)
	}

	got, err := DecodePack(packed, originals)
	if err != nil {
		t.Fatalf("DecodePack() error = %v", err)
	}
	if strings.Join(got, "||") != strings.Join(originals, "||") {
		t.Fatalf("decoded = %#v, want %#v", got, originals)
	}
}

func TestDecodePackRejectsMissingMarker(t *testing.T) {
	t.Parallel()

	_, err := DecodePack("no markers here", []string{"a", "b"})
	if err == nil {
		t.Fatal("DecodePack() error = nil, want missing marker")
	}
}
