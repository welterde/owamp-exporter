package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Config struct {
	targets      map[string]TargetCfg
	measurements []MeasurementCfg
	portRangeMin uint64
	portRangeMax uint64
	baseWorkDir  string
	powstreamCmd string
}

type TargetCfg struct {
	hostname  string
	local     bool
	shortname string
	afi6      bool
}

type MeasurementCfg struct {
	targetSrc   string
	targetDst   string
	pps         uint64
	duration    uint64
	tags        []string
	bucketWidth string
	promHistBins []float64
}

func ParseConfig(r *bufio.Reader) (Config, error) {
	var err error
	ret := Config{
		targets:      make(map[string]TargetCfg),
		measurements: make([]MeasurementCfg, 0, 3),
		powstreamCmd: "powstream",
		portRangeMin: 9000,
		portRangeMax: 9999,
	}

	s := bufio.NewScanner(r)

	// default settings
	var defaultPPS uint64 = 10
	var defaultDuration uint64 = 60          // s
	var defaultBucketWidth string = "0.0001" // s

	// default settings for prometheus histogram
	var defaultHistMinLatency uint64 = 1 // ms
	var defaultHistMaxLatency uint64 = 1000 // ms
	var defaultHistMaxLinearLatency uint64 = 50 // ms
	var defaultHistLinearPtsPerMs uint64 = 4
	var defaultHistLogPts uint64 = 5

	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		// skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "TARGET":
			if len(parts) < 3 {
				return ret, errors.New("Config syntax error: TARGET <shortname> <hostname> [options]")
			}

			target := TargetCfg{
				hostname:  parts[2],
				local:     false,
				shortname: parts[1],
				afi6:      net.ParseIP(parts[2]).To4() == nil,
			}
			for _, option := range parts[3:] {
				if option == "local" {
					target.local = true
				} else if shortname, found := strings.CutPrefix(option, "shortname="); found {
					target.shortname = shortname
				}
			}
			ret.targets[parts[1]] = target

		case "DEFAULT-PPS":
			if len(parts) != 2 {
				return ret, errors.New("Config syntax error: DEFAULT-PPS <pps-value>")
			}
			if defaultPPS, err = strconv.ParseUint(parts[1], 10, 64); err != nil {
				return ret, errors.New("Config syntax error: DEFAULT-PPS invalid int")
			}

		case "DEFAULT-HIST":
			if len(parts) != 3 {
				return ret, errors.New("Config syntax error: DEFAULT-HIST <option> <value>")
			}
			
			switch strings.ToLower(parts[1]) {
			case "min-latency":
				if defaultHistMinLatency, err = strconv.ParseUint(parts[2], 10, 64); err != nil {
					return ret, errors.New("Config syntax error: DEFAULT-HIST min-latency <integer>: invalid int")
				}
			case "max-linear-latency":
				if defaultHistMaxLinearLatency, err = strconv.ParseUint(parts[2], 10, 64); err != nil {
					return ret, errors.New("Config syntax error: DEFAULT-HIST max-linear-latency <integer>: invalid int")
				}
			case "max-latency":
				if defaultHistMaxLatency, err = strconv.ParseUint(parts[2], 10, 64); err != nil {
					return ret, errors.New("Config syntax error: DEFAULT-HIST max-latency <integer>: invalid int")
				}
			case "linear-points-per-ms":
				if defaultHistLinearPtsPerMs, err = strconv.ParseUint(parts[2], 10, 64); err != nil {
					return ret, errors.New("Config syntax error: DEFAULT-HIST linear-points-per-ms <integer>: invalid int")
				}
			case "log-points":
				if defaultHistLogPts, err = strconv.ParseUint(parts[2], 10, 64); err != nil {
					return ret, errors.New("Config syntax error: DEFAULT-HIST log-points <integer>: invalid int")
				}
			}

		case "MEASUREMENT":
			if len(parts) < 3 {
				return ret, errors.New("Config syntax error: MEASUREMENT <targetSRC> <targetDST> [options]")
			}

			var afi string
			if ret.targets[parts[1]].afi6 {
				afi = "ip6"
			} else {
				afi = "ip4"
			}

			// copy the default settings for the prometheus histogram
			histMinLatency := defaultHistMinLatency // ms
			histMaxLatency := defaultHistMaxLatency // ms
			histMaxLinearLatency := defaultHistMaxLinearLatency // ms
			histLinearPtsPerMs := defaultHistLinearPtsPerMs
			histLogPts := defaultHistLogPts

			measurement := MeasurementCfg{
				targetSrc:   parts[1],
				targetDst:   parts[2],
				pps:         defaultPPS,
				duration:    defaultDuration,
				bucketWidth: defaultBucketWidth,
				tags: []string{
					fmt.Sprintf("src_short_name=\"%s\"", parts[1]),
					fmt.Sprintf("dst_short_name=\"%s\"", parts[2]),
					fmt.Sprintf("src_hostname=\"%s\"", ret.targets[parts[1]].hostname),
					fmt.Sprintf("dst_hostname=\"%s\"", ret.targets[parts[2]].hostname),
					fmt.Sprintf("afi=\"%s\"", afi),
				},
			}
			for _, option := range parts[3:] {
				if suffix, found := strings.CutPrefix(option, "pps="); found {
					if measurement.pps, err = strconv.ParseUint(suffix, 10, 64); err != nil {
						return ret, errors.New("Config syntax error: MEASUREMENT pps value not integer")
					}
				}
				if suffix, found := strings.CutPrefix(option, "bucketwidth="); found {
					measurement.bucketWidth = suffix
				}
				if suffix, found := strings.CutPrefix(option, "hist-min-latency="); found {
					if histMinLatency, err = strconv.ParseUint(suffix, 10, 64); err != nil {
						return ret, errors.New("Config syntax error: MEASUREMENT hist-min-latency value not integer")
					}
				}
				if suffix, found := strings.CutPrefix(option, "hist-max-linear-latency="); found {
					if histMaxLinearLatency, err = strconv.ParseUint(suffix, 10, 64); err != nil {
						return ret, errors.New("Config syntax error: MEASUREMENT hist-max-linear-latency value not integer")
					}
				}

			}
			measurement.promHistBins = MakePromHistBins(histMinLatency, histMaxLatency, histMaxLinearLatency, histLinearPtsPerMs, histLogPts)
			ret.measurements = append(ret.measurements, measurement)
		}

	}
	return ret, nil
}
