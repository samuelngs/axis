package parser

import (
	"errors"
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/samuelngs/axis/models"
)

// OpenFile - open file and return content in bytes
func OpenFile(filename string) ([]byte, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("unable read configuration file")
	}
	return data, nil
}

// OpenYaml - open yaml file and parse yaml content
func OpenYaml(filename string) (yaml *models.YamlOptions, e error) {
	data, err := OpenFile(filename)
	if err != nil {
		e = err
		return
	}
	conf, err := ParseYaml(data)
	if err != nil {
		e = err
		return
	}
	return conf, nil
}

// ParseYaml to parse yaml file to create models.YamlOptions
func ParseYaml(data []byte) (*models.YamlOptions, error) {
	opts := &models.YamlOptions{}
	err := yaml.Unmarshal(data, &opts)
	if err != nil {
		fmt.Println(err)
		return nil, errors.New("cannot parse configuration file")
	}
	// verify configuration
	if err := opts.Verify(); err != nil {
		return nil, err
	}
	// apply default configurations
	opts.ApplyDefault()
	return opts, err
}
