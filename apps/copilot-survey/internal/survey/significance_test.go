package survey

import (
	"math"
	"testing"
)

// The wants are from an independent Python reference that ranks the
// expanded answers one by one (no scipy).
func TestMannWhitney(t *testing.T) {
	for _, tc := range []struct {
		a, b map[int]int
		want float64
	}{
		{map[int]int{1: 1, 2: 1, 3: 1}, map[int]int{4: 1, 5: 1, 6: 1}, 0.049534613435626755}, // no ties: z = 4.5/√5.25
		{map[int]int{1: 2, 2: 3, 3: 5, 4: 4, 5: 1}, map[int]int{2: 1, 3: 2, 4: 6, 5: 6}, 0.005322640537191904},
		{map[int]int{5: 2, 4: 1, 2: 1}, map[int]int{3: 5}, 0.17752985241215352},
	} {
		got, ok := mannWhitney(tc.a, tc.b)
		if !ok || math.Abs(got-tc.want) > 1e-12 {
			t.Errorf("mannWhitney(%v, %v) = %v %v, want %v", tc.a, tc.b, got, ok, tc.want)
		}
		if back, _ := mannWhitney(tc.b, tc.a); math.Abs(back-got) > 1e-12 {
			t.Errorf("not symmetric: %v vs %v", back, got)
		}
	}
	for _, tc := range [][2]map[int]int{{{3: 4}, {3: 2}}, {{}, {3: 2}}} {
		if _, ok := mannWhitney(tc[0], tc[1]); ok {
			t.Errorf("mannWhitney(%v, %v): want no test", tc[0], tc[1])
		}
	}
}

func TestHolm(t *testing.T) {
	// Sorted 0.005·5, 0.01·4, 0.03·3, then 0.04·2 = 0.08 held up to 0.09, 0.9·1.
	got := holm([]float64{0.01, 0.04, 0.03, 0.005, 0.9})
	want := []float64{0.04, 0.09, 0.09, 0.025, 0.9}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("holm = %v, want %v", got, want)
		}
	}
}
