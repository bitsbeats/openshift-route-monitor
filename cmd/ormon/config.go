package main

import (
	"os"

	"gopkg.in/yaml.v2"

	"github.com/bitsbeats/openshift-route-monitor/internal/kube"
	"github.com/bitsbeats/openshift-route-monitor/internal/monitor"
)

type config struct {
	Targets []kube.Config  `yaml:"targets"`
	Monitor monitor.Config `yaml:"monitor"`
}

// loadConfig loads the configuration
func loadConfig() (c *config, err error) {
	configFile := "/etc/openshift-route-exporter/config.yml"
	if configFileDirFromEnv, ok := os.LookupEnv("CONFIG"); ok {
		configFile = configFileDirFromEnv
	}
	r, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	c = &config{}
	err = yaml.NewDecoder(r).Decode(c)
	if err != nil {
		return nil, err
	}
	return c, nil
}
