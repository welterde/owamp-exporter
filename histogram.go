package main

import (
	"bufio"
	"fmt"
	"math"
	"sort"
	"sync"
)

// prometheus histogram code written from scratch

// VictoriaMetrics code based on https://github.com/VictoriaMetrics/metrics/blob/master/histogram.go
// which is licensed under MIT and authored by valyala, tenmozes, hagen1778

// dump out histogram in prometheus style
func WriteHistogramPrometheus(w *bufio.Writer, name string, tags string, timestamp uint64, histo []HistogramEntry, scale float64) error {
	// create sorted copy of input histogram
	hist := make([]HistogramEntry, len(histo))
	_ = copy(hist, histo)
	sort.Slice(hist, func(i int, j int) bool {
		return hist[i].key < hist[j].key
	})

	// not useful to show empty histogram.. and code below is not ready for that either
	if len(hist) < 1 {
		return nil
	}

	// now produce a cumulative histogram from the existing one
	var cumsum float64 = 0.0
	for i, entry := range hist {
		cumsum += float64(entry.value) * float64(entry.key) * scale
		if (i + 1) < len(hist) {
			hist[i+1].value += entry.value
		}
	}

	for i, entry := range hist {
		var le string
		if (i + 1) < len(hist) {
			le = fmt.Sprintf("%e", float64(entry.key)*scale)
		} else {
			le = "+Inf"
		}
		line := fmt.Sprintf("%s_bucket{%s,le=\"%s\"} %d %d\n", name, tags, le, entry.value, timestamp)
		_, err := w.WriteString(line)
		if err != nil {
			return err
		}
	}

	line := fmt.Sprintf("%s_sum{%s} %e %d\n", name, tags, cumsum, timestamp)
	_, err := w.WriteString(line)
	if err != nil {
		return err
	}

	line = fmt.Sprintf("%s_count{%s} %d %d\n", name, tags, hist[len(hist)-1].value, timestamp)
	_, err = w.WriteString(line)
	if err != nil {
		return err
	}

	return nil
}

const (
	e10Min              = -9
	e10Max              = 18
	bucketsPerDecimal   = 18
	decimalBucketsCount = e10Max - e10Min
	bucketsCount        = decimalBucketsCount * bucketsPerDecimal
)

var bucketMultiplier = math.Pow(10, 1.0/bucketsPerDecimal)

var (
	lowerBucketRange = fmt.Sprintf("0...%.3e", math.Pow10(e10Min))
	upperBucketRange = fmt.Sprintf("%.3e...+Inf", math.Pow10(e10Max))

	bucketRanges     [bucketsCount]string
	bucketRangesOnce sync.Once
)

func initBucketRanges() {
	v := math.Pow10(e10Min)
	start := fmt.Sprintf("%.3e", v)
	for i := 0; i < bucketsCount; i++ {
		v *= bucketMultiplier
		end := fmt.Sprintf("%.3e", v)
		bucketRanges[i] = start + "..." + end
		start = end
	}
}

func getVMRange(bucketIdx int) string {
	bucketRangesOnce.Do(initBucketRanges)
	return bucketRanges[bucketIdx]
}

type VictoHist struct {
	decimalBuckets [decimalBucketsCount]*[bucketsPerDecimal]uint64

	lower uint64
	upper uint64

	sum float64
}

// VisitNonZeroBuckets calls f for all buckets with non-zero counters.
//
// vmrange contains "<start>...<end>" string with bucket bounds. The lower bound
// isn't included in the bucket, while the upper bound is included.
// This is required to be compatible with Prometheus-style histogram buckets
// with `le` (less or equal) labels.
func (h *VictoHist) VisitNonZeroBuckets(f func(vmrange string, count uint64)) {
	if h.lower > 0 {
		f(lowerBucketRange, h.lower)
	}
	for decimalBucketIdx, db := range h.decimalBuckets[:] {
		if db == nil {
			continue
		}
		for offset, count := range db[:] {
			if count > 0 {
				bucketIdx := decimalBucketIdx*bucketsPerDecimal + offset
				vmrange := getVMRange(bucketIdx)
				f(vmrange, count)
			}
		}
	}
	if h.upper > 0 {
		f(upperBucketRange, h.upper)
	}
}

func WriteHistogramVictoriaMetrics(w *bufio.Writer, name string, tags string, timestamp uint64, histo []HistogramEntry, scale float64) error {
	// rebin all the histogram bins into victoriametrics histogram
	hist := VictoHist{}
	for _, entry := range histo {
		v := float64(entry.key)
		count := entry.value
		bucketIdx := (math.Log10(v) - e10Min) * bucketsPerDecimal
		hist.sum += v * float64(count)
		if bucketIdx < 0 {
			hist.lower += count
		} else if bucketIdx >= bucketsCount {
			hist.upper += count
		} else {
			idx := uint(bucketIdx)
			if bucketIdx == float64(idx) && idx > 0 {
				// Edge case for 10^n values, which must go to the lower bucket
				// according to Prometheus logic for `le`-based histograms.
				idx--
			}
			decimalBucketIdx := idx / bucketsPerDecimal
			offset := idx % bucketsPerDecimal
			db := hist.decimalBuckets[decimalBucketIdx]
			if db == nil {
				var b [bucketsPerDecimal]uint64
				db = &b
				hist.decimalBuckets[decimalBucketIdx] = db
			}
			db[offset] += count
		}
	}

	// now output all non-zero buckets
	countTotal := uint64(0)
	hist.VisitNonZeroBuckets(func(vmrange string, count uint64) {
		fmt.Fprintf(w, "%s_bucket{%s,vmrange=%q} %d %d\n", name, tags, vmrange, count, timestamp)
		countTotal += count
	})

	// just quit if we didn't output anything
	if countTotal == 0 {
		return nil
	}

	// output sum
	fmt.Fprintf(w, "%s_sum{%s} %g %d\n", name, tags, hist.sum, timestamp)

	// output count
	fmt.Fprintf(w, "%s_count{%s} %d %d\n", name, tags, countTotal, timestamp)

	return nil
}
