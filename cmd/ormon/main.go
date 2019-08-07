package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/bitsbeats/openshift-route-monitor/internal/kube"
	"github.com/bitsbeats/openshift-route-monitor/internal/monitor"
)

func main() {
	// exit handler
	errs := make(chan error, 1)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// config
	config, err := loadConfig()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infoln("loaded config")

	// start watchers
	mw, err := kube.NewMultiWatcher(config.Targets)
	if err != nil {
		logrus.Fatal(err)
	}
	go mw.Watch(ctx)
	logrus.Infoln("started watchers")

	// start monitor
	monitor, err := monitor.New(config.Monitor, mw)
	if err != nil {
		logrus.Fatal(err)
	}
	monitor.Run(ctx, errs)
	logrus.Infoln("ran monitor")

	// exit handler
	select {
	case err := <-errs:
		logrus.Fatal(err)
	case s := <-sig:
		logrus.Printf("received signal %s, stopping", s.String())
	}
	logrus.Infoln("bye")
}
