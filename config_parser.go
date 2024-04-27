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

			}
			ret.measurements = append(ret.measurements, measurement)
		}

	}
	return ret, nil
}