package main

import (
	"bufio"
	"errors"
	"strconv"
	"strings"
)

type SummaryReport struct {
	startTime float64
	endTime   float64

	sentPkts uint64
	dupPkts  uint64
	lostPkts uint64

	maxErr float64

	latencyMin float64
	latencyMax float64
	latencyMed float64

	latencyHistWidth float64
	latencyHist      []HistogramEntry
	ttlHist          []HistogramEntry
	reorderingHist   []HistogramEntry
}

type HistogramEntry struct {
	key   uint64
	value uint64
}

// convert a OWAMP/owstats timestamp into a milliseconds UNIX time suitable for prometheus/openmetrics
// the input time stamp has the upper 32bit being the unix timestamp
// and the lower 32bit the fractional time
func parseOWTimestamp(t uint64) uint64 {
	// upper 32bit converted to milliseconds
	upper := (t >> 32) * 1000

	// TODO: decode the lower 32bit and convert to milliseconds
	return upper
}

func ParseHistogram(s *bufio.Scanner) ([]HistogramEntry, error) {
	data := make([]HistogramEntry, 0, 30)

	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		// last line is something like </BUCKETS>, </TTLBUCKETS>, ... - so check for </
		if strings.HasPrefix(line, "</") {
			return data, nil
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			return data, errors.New("invalid histogram. Syntax error; invalid entry length.")
		}
		key, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return data, errors.New("invalid histogram. Syntax error in key.")
		}
		value, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return data, errors.New("invalid histogram. Syntax error in value.")
		}
		data = append(data, HistogramEntry{key, value})
	}

	return data, errors.New("incomplete histogram. Missing closing bracket.")
}

func ParseSummary(r *bufio.Reader) (SummaryReport, error) {
	var err error
	ret := SummaryReport{}

	s := bufio.NewScanner(r)

	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		parts := strings.Fields(line)
		var entryValue string
		if len(parts) == 0 {
			continue
		} else if len(parts) == 2 {
			entryValue = parts[1]
		} else if len(parts) == 1 {
			entryValue = ""
		} else {
			continue
		}

		switch parts[0] {
		case "UNIX_START_TIME":
			if ret.startTime, err = strconv.ParseFloat(entryValue, 64); err != nil {
				return ret, errors.New("Invalid start_time. Parse error")
			}
			//ret.startTime = parseOWTimestamp(ret.startTime)
		case "UNIX_END_TIME":
			if ret.endTime, err = strconv.ParseFloat(entryValue, 64); err != nil {
				return ret, errors.New("Invalid end_time. Parse error")
			}
			//ret.endTime = parseOWTimestamp(ret.endTime)
		case "SENT":
			if ret.sentPkts, err = strconv.ParseUint(entryValue, 10, 64); err != nil {
				return ret, errors.New("Invalid sent pkts. Parse error")
			}
		case "DUPS":
			if ret.dupPkts, err = strconv.ParseUint(entryValue, 10, 64); err != nil {
				return ret, errors.New("Invalid dup pkts. Parse error")
			}
		case "LOST":
			if ret.lostPkts, err = strconv.ParseUint(entryValue, 10, 64); err != nil {
				return ret, errors.New("Invalid lost pkts. Parse error")
			}
		case "MAXERR":
			if ret.maxErr, err = strconv.ParseFloat(entryValue, 64); err != nil {
				return ret, errors.New("Invalid maxerr. Parse error")
			}
		case "MIN":
			if ret.latencyMin, err = strconv.ParseFloat(entryValue, 64); err != nil {
				return ret, errors.New("Invalid min. Parse error")
			}
		case "MEDIAN":
			if ret.latencyMed, err = strconv.ParseFloat(entryValue, 64); err != nil {
				return ret, errors.New("Invalid median. Parse error")
			}
		case "MAX":
			if ret.latencyMax, err = strconv.ParseFloat(entryValue, 64); err != nil {
				return ret, errors.New("Invalid max. Parse error")
			}
		case "BUCKET_WIDTH":
			if ret.latencyHistWidth, err = strconv.ParseFloat(entryValue, 64); err != nil {
				return ret, errors.New("Invalid bucket_width. Parse error")
			}
		case "<BUCKETS>":
			if ret.latencyHist, err = ParseHistogram(s); err != nil {
				return ret, err
			}

		case "<TTLBUCKETS>":
			if ret.ttlHist, err = ParseHistogram(s); err != nil {
				return ret, err
			}
		case "<NREORDERING>":
			if ret.reorderingHist, err = ParseHistogram(s); err != nil {
				return ret, err
			}
		}
	}

	return ret, nil
}
