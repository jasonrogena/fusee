package main

import (
	"flag"

	"github.com/jasonrogena/fusee/internal/app/fusee/config"
	"github.com/jasonrogena/fusee/internal/app/fusee/mount"
	log "github.com/sirupsen/logrus"
)

func main() {
	debug := flag.Bool("debug", false, "Print debug data")
	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatal("Usage:\n  fusee <path to the configuration>")
	}
	config, configErr := config.NewConfig(flag.Arg(0))
	if configErr != nil {
		log.Fatal(configErr.Error())
		return
	}
	if *debug {
		log.SetLevel(log.DebugLevel)
	}
	for curMountName, curMountConf := range config.Mounts {
		curMount := mount.NewRoot(curMountName, curMountConf)
		mountErr := curMount.Mount(*debug)
		if mountErr != nil {
			log.Error(mountErr.Error())
		}
	}
}
