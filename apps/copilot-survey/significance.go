package main

import (
	"cmp"
	"maps"
	"math"
	"slices"
	"strconv"
)

// significance tests the first wave against the last for one scale or
// matrix item. Only set where both waves answered the same single version.
type significance struct {
	// P: two-sided Mann–Whitney U, normal approximation with tie correction
	// and without continuity correction.
	P *float64 `json:"p,omitempty"`
	// PHolm: P after Holm correction over every tested item in the comparison.
	PHolm *float64 `json:"p_holm,omitempty"`
	// CILow, CIHigh: 95 % interval for last mean minus first mean (Welch,
	// normal quantile 1.96; fine at survey sizes, too narrow under ~30).
	CILow  *float64 `json:"ci_low,omitempty"`
	CIHigh *float64 `json:"ci_high,omitempty"`
}

// testFirstLast sets sig from ws unless the item has a break or either end
// wave has nothing to test. It reports whether it set anything.
func testFirstLast(sig *significance, ws []waveCounts, br bool) bool {
	if br || len(ws) < 2 || len(ws[0].Versions) != 1 || len(ws[len(ws)-1].Versions) != 1 {
		return false
	}
	a, b := scaleCounts(ws[0].Versions[0]), scaleCounts(ws[len(ws)-1].Versions[0])
	p, ok := mannWhitney(a, b)
	if !ok {
		return false
	}
	lo, hi := welchCI(a, b)
	sig.P, sig.CILow, sig.CIHigh = &p, &lo, &hi
	return true
}

func scaleCounts(vr waveStat) map[int]int {
	m := map[int]int{}
	for _, o := range vr.Options {
		if v, err := strconv.Atoi(o.Value); err == nil && o.Count != nil && *o.Count > 0 {
			m[v] += *o.Count
		}
	}
	return m
}

// mannWhitney is the two-sided p of the Mann–Whitney U test for two count
// tables. ok is false when a side is empty or every answer is the same.
func mannWhitney(a, b map[int]int) (float64, bool) {
	var n1, n2 float64
	for _, c := range a {
		n1 += float64(c)
	}
	for _, c := range b {
		n2 += float64(c)
	}
	n := n1 + n2
	if n1 == 0 || n2 == 0 {
		return 0, false
	}
	all := maps.Clone(a)
	for v, c := range b {
		all[v] += c
	}
	var below, r1, ties float64
	for _, v := range sortedKeys(all) {
		t := float64(all[v])
		r1 += float64(a[v]) * (below + (t+1)/2) // midrank
		below += t
		ties += t*t*t - t
	}
	u := r1 - n1*(n1+1)/2
	variance := n1 * n2 / 12 * (n + 1 - ties/(n*(n-1)))
	if variance <= 0 {
		return 0, false
	}
	z := math.Abs(u-n1*n2/2) / math.Sqrt(variance)
	return math.Erfc(z / math.Sqrt2), true
}

// welchCI is the 95 % interval for mean(b) - mean(a).
func welchCI(a, b map[int]int) (lo, hi float64) {
	moments := func(m map[int]int) (n, mean, variance float64) {
		var sum float64
		for v, c := range m {
			n, sum = n+float64(c), sum+float64(v*c)
		}
		mean = sum / n
		for v, c := range m {
			variance += float64(c) * (float64(v) - mean) * (float64(v) - mean)
		}
		if n > 1 {
			variance /= n - 1
		}
		return n, mean, variance
	}
	na, ma, va := moments(a)
	nb, mb, vb := moments(b)
	d, se := mb-ma, math.Sqrt(va/na+vb/nb)
	return d - 1.96*se, d + 1.96*se
}

// holm is Holm's step-down adjustment of ps, in the input order.
func holm(ps []float64) []float64 {
	order := make([]int, len(ps))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(i, j int) int { return cmp.Compare(ps[i], ps[j]) })
	out := make([]float64, len(ps))
	running := 0.0
	for k, i := range order {
		running = math.Max(running, math.Min(1, float64(len(ps)-k)*ps[i]))
		out[i] = running
	}
	return out
}
