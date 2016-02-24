package parser

import (
	"errors"

	"gopkg.in/yaml.v2"
)

type (
	// EtcdOptions is the `etcd` section in yaml configuration file
	EtcdOptions struct {
		Endpoints []string `yaml:"endpoints"`
	}
	// DaemonOptions is the `daemon` section in yaml configuration file
	DaemonOptions struct {
		Discovery  string   `yaml:"discovery"`
		EntryPoint string   `yaml:"entrypoint"`
		Master     []string `yaml:"master"`
		Slave      []string `yaml:"slave"`
	}
	// YamlOptions is the content of yaml configuration file
	YamlOptions struct {
		Etcd   *EtcdOptions
		Daemon *DaemonOptions
	}
)

// ParseYaml to parse yaml file to create YamlOptions
func ParseYaml(data []byte) (*YamlOptions, error) {
	opts := &YamlOptions{}
	err := yaml.Unmarshal(data, &opts)
	if err != nil {
		return nil, errors.New("cannot parse configuration file")
	}
	return opts, err
}
