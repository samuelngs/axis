package models

import "fmt"

type (
	// EtcdOptions is the `etcd` section in yaml configuration file
	EtcdOptions struct {
		Endpoints []string `yaml:"endpoints,omitempty"`
	}
	// ApplicationOptions is the `daemon` section in yaml configuration file
	ApplicationOptions struct {
		Name   string                 `yaml:"name,omitempty"`
		Prefix string                 `yaml:"prefix,omitempty"`
		Leader *ApplicationEntryPoint `yaml:"leader,omitempty"`
		Worker *ApplicationEntryPoint `yaml:"worker,omitempty"`
	}
	// ApplicationEntryPoint is the `daemon` entry point configuration
	ApplicationEntryPoint struct {
		EntryPoint string             `yaml:"entrypoint"`
		Command    []string           `yaml:"command"`
		Health     *ApplicationHealth `yaml:"health,omitempty"`
	}
	// ApplicationHealth is the `health checks` section for the `daemon`
	ApplicationHealth struct {
		Port []uint `yaml:"port"`
	}
	// YamlOptions is the content of yaml configuration file
	YamlOptions struct {
		Etcd   *EtcdOptions
		Daemon *ApplicationOptions
	}
)

var (
	// DefaultEtcdEndpoints - the default etcd server access endpoint
	DefaultEtcdEndpoints = []string{"http://127.0.0.1:2379", "http://127.0.0.1:4001"}
)

// ApplyDefault - default configuration
func (yaml *YamlOptions) ApplyDefault() {
	if yaml.Etcd == nil {
		yaml.Etcd = &EtcdOptions{DefaultEtcdEndpoints}
	}
	if yaml.Etcd.Endpoints == nil {
		yaml.Etcd = &EtcdOptions{DefaultEtcdEndpoints}
	}
	if yaml.Daemon.Prefix == "" {
		yaml.Daemon.Prefix = "/"
	}
}

// Verify - to verify yaml configuration
func (yaml *YamlOptions) Verify() error {
	if yaml.Daemon == nil {
		return fmt.Errorf("require the daemon section")
	}
	if yaml.Daemon.Leader == nil {
		return fmt.Errorf("require the master entrypoint")
	}
	if yaml.Daemon.Worker == nil {
		return fmt.Errorf("require the worker entrypoint")
	}
	if yaml.Daemon.Name == "" {
		return fmt.Errorf("require the service name")
	}
	return nil
}
