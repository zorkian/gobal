/*
	gobal - gobal.go

	An HTTP load balancer, written in Go. This is based off of the ideas and
	learnings made by writing and working on Perlbal.

	Copyright (c) 2012-2013 by authors and contributors.
*/

package main

import (
	"flag"
	logging "github.com/fluffle/golog/logging"
	"os"
	"time"
)

var log logging.Logger

func main() {
	var conf = flag.String("config-file", "", "configuration file to load")
	flag.Parse()

	log = logging.InitFromFlags()
	log.Info("gobal starting up!")

	err := loadConfig(*conf)
	if err != nil {
		log.Error("failed: %s", err)
		os.Exit(1)
	}

	// Loading the configuration file will have started is up and everything
	// we should be doing. Now: do nothing.
	for {
		time.Sleep(60 * time.Second)
	}
}
