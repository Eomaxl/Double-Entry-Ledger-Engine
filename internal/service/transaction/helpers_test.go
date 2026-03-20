package transaction

import "testing"

func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		out  []string
	}{
		{name: "nil input", in: nil, out: nil},
		{name: "empty input", in: []string{}, out: nil},
		{name: "single", in: []string{"a"}, out: []string{"a"}},
		{name: "duplicates", in: []string{"a", "a", "b", "b", "c"}, out: []string{"a", "b", "c"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := uniqueStrings(tc.in)
			if len(got) != len(tc.out) {
				t.Fatalf("length mismatch got=%v want=%v", got, tc.out)
			}
			for i := range got {
				if got[i] != tc.out[i] {
					t.Fatalf("item %d mismatch got=%s want=%s", i, got[i], tc.out[i])
				}
			}
		})
	}
}
