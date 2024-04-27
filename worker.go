package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Worker struct {
	measurementOut chan MeasurementReport
	measurementIdx uint
	workDir        string
	cfg            Config
	mcfg           MeasurementCfg
}

type MeasurementReport struct {
	measurementIdx   uint
	summary          SummaryReport
	metricsTimestamp float64
}

func NewWorker(cfg Config, idx uint, outCh chan MeasurementReport) *Worker {
	mcfg := cfg.measurements[idx]

	workDir := filepath.Join(cfg.baseWorkDir, fmt.Sprintf("%s_%s", mcfg.targetSrc, mcfg.targetDst))
	err := os.MkdirAll(workDir, 0750)
	if err != nil {
		log.Fatal(err)
	}

	return &Worker{
		workDir:        workDir,
		measurementOut: outCh,
		measurementIdx: idx,
		cfg:            cfg,
		mcfg:           mcfg,
	}
}

func printErrorMsgs(instancePrefix string, r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		log.Printf("%s stderr: %s", instancePrefix, line)
	}
}

func (w *Worker) RunWorker() {
	srcHostname := w.cfg.targets[w.mcfg.targetSrc].hostname
	destHostname := w.cfg.targets[w.mcfg.targetDst].hostname

	pktCount := w.mcfg.duration * w.mcfg.pps
	pktInterval := 1 / float64(w.mcfg.pps)

	cmdArgs := []string{
		// direction is from client to server
		"-t",
		// number of packets
		"-c",
		fmt.Sprintf("%d", pktCount),
		// interval between packets
		"-i",
		fmt.Sprintf("%.6f", pktInterval),
		// port range
		"-P",
		fmt.Sprintf("%d-%d", w.cfg.portRangeMin, w.cfg.portRangeMax),
		// destination directory for output files
		"-d",
		w.workDir,
		// when generating the input histogram use this bucket width
		"-b",
		w.mcfg.bucketWidth,
		// output the filenames of the generated output files on stdout
		"-p",
		// output the UNIX timestamp as well
		"-U",
		// destination host
		destHostname,
	}

	// in case the src host is not local we have to specify the remote owampd server
	// as additional optional argument to powstream
	if !w.cfg.targets[w.mcfg.targetSrc].local {
		cmdArgs = append(cmdArgs, srcHostname)
	}

	log.Printf("Running %v", cmdArgs)

	cmd := exec.Command(w.cfg.powstreamCmd, cmdArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("%d failed start stdout-pipe: %v", w.measurementIdx, err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("%d failed start stderr-pipe: %v", w.measurementIdx, err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("%d failed start: %v", w.measurementIdx, err)
		return
	}

	s := bufio.NewScanner(stdout)
	go printErrorMsgs("powstream", stderr)

	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		if strings.HasSuffix(line, ".sum") {
			// launch process to parse the file
			go ParseSummaryFile(w.measurementOut, w.measurementIdx, line)
		}
	}

	// if we are here means the process must have exited.. so restart after some delay
	time.Sleep(30 * time.Second)
	w.RunWorker()
}

func ParseSummaryFile(out chan MeasurementReport, idx uint, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("%d failed read of %s: %v", idx, path, err)
		return
	}
	r := bufio.NewReader(bytes.NewReader(data))
	summary, err := ParseSummary(r)
	if err != nil {
		log.Printf("%d failed parse of %s: %v", idx, path, err)
	}
	out <- MeasurementReport{
		measurementIdx:   idx,
		summary:          summary,
		metricsTimestamp: (summary.startTime + summary.endTime) / 2,
	}
}
