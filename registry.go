package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"
)

type Registry struct {
	reports           map[uint]MeasurementReport
	inChannel         chan MeasurementReport
	cfg               Config
	mutex             sync.Mutex
	victoriaHistogram bool
}

func NewRegistry(cfg Config) *Registry {
	reg := &Registry{
		reports:   make(map[uint]MeasurementReport),
		inChannel: make(chan MeasurementReport),
		cfg:       cfg,
	}
	go reg.runCollector()
	return reg
}

func (r *Registry) runCollector() {
	for {
		report := <-r.inChannel
		r.mutex.Lock()
		r.reports[report.measurementIdx] = report
		r.mutex.Unlock()
	}
}

func (r *Registry) DumpMetrics(w io.Writer) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	bw := bufio.NewWriterSize(w, 512*1024)
	defer bw.Flush()

	for mIdx, report := range r.reports {
		mcfg := r.cfg.measurements[mIdx]
		tags := strings.Join(mcfg.tags, ",")
		ts := uint64(report.metricsTimestamp * 1000.0)
		rs := report.summary

		var err error

		// write run meta-data
		_, err = fmt.Fprintf(bw, "owamp_start_time{%s} %.3f %d\n", tags, rs.startTime, ts)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(bw, "owamp_end_time{%s} %.3f %d\n", tags, rs.endTime, ts)
		if err != nil {
			return err
		}

		// write packet stats
		_, err = fmt.Fprintf(bw, "owamp_packets_sent{%s} %d %d\n", tags, rs.sentPkts, ts)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(bw, "owamp_packets_dup{%s} %d %d\n", tags, rs.dupPkts, ts)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(bw, "owamp_packets_lost{%s} %d %d\n", tags, rs.lostPkts, ts)
		if err != nil {
			return err
		}

		// write latency histogram
		if r.victoriaHistogram {
			err = WriteHistogramVictoriaMetrics(bw, "owamp_latency", tags, ts, rs.latencyHist, rs.latencyHistWidth)
			if err != nil {
				return err
			}

			// write TTL histogram
			err = WriteHistogramVictoriaMetrics(bw, "owamp_ttl", tags, ts, rs.ttlHist, 1.0)
			if err != nil {
				return err
			}

			// write reordering histogram
			err = WriteHistogramVictoriaMetrics(bw, "owamp_reordering", tags, ts, rs.reorderingHist, 1.0)
			if err != nil {
				return err
			}
		} else {
			err = WriteHistogramPrometheus(bw, "owamp_latency", tags, ts, rs.latencyHist, rs.latencyHistWidth, mcfg.promHistBins)
			if err != nil {
				return err
			}

			// TODO: write TTL histogram
			// err = WriteHistogramPrometheus(bw, "owamp_ttl", tags, ts, rs.ttlHist, 1.0)
			// if err != nil {
			// 	return err
			// }

			// TODO: write reordering histogram
			// err = WriteHistogramPrometheus(bw, "owamp_reordering", tags, ts, rs.reorderingHist, 1.0)
			// if err != nil {
			// 	return err
			// }
		}

		// write latency summary values
		_, err = fmt.Fprintf(bw, "owamp_latency_min{%s} %e %d\n", tags, rs.latencyMin, ts)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(bw, "owamp_latency_median{%s} %e %d\n", tags, rs.latencyMed, ts)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(bw, "owamp_latency_max{%s} %e %d\n", tags, rs.latencyMax, ts)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(bw, "owamp_time_error_estimate{%s} %e %d\n", tags, rs.maxErr, ts)
		if err != nil {
			return err
		}
	}
	return nil
}
