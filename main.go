package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

var configFile = flag.String("cfg-file", "owamp-export.cfg", "The configuration file")
var listenPort = flag.Uint("listen-port", 9099, "Listen port for exporter")
var powstreamCmd = flag.String("powstream-cmd", "powstream", "Location of powstream binary to use")
var workDir = flag.String("workdir", "", "Location to place collected owping reports")
var victoriaHistogram = flag.Bool("victoria-histogram", false, "Use the VictoriaMetrics histogram format")

func main() {
	flag.Parse()

	// read configuration file
	data, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	cr := bufio.NewReader(bytes.NewReader(data))
	cfg, err := ParseConfig(cr)
	if err != nil {
		log.Fatal(err)
	}

	// override some config things
	cfg.powstreamCmd = *powstreamCmd
	if *workDir != "" {
		cfg.baseWorkDir = *workDir
	}

	// launch registry
	reg := NewRegistry(cfg)
	reg.victoriaHistogram = *victoriaHistogram

	// launch workers
	for idx, _ := range cfg.measurements {
		w := NewWorker(cfg, uint(idx), reg.inChannel)
		go w.RunWorker()
	}

	http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
		reg.DumpMetrics(w)
	})
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *listenPort), nil))
}
