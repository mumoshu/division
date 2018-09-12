package framework

import (
	"encoding/json"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/mumoshu/division/api"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

func WriteToStdout(message string) {
	os.Stdout.WriteString(message + "\n")
}

func WriteToStderr(message string) {
	println(message)
}

func LoadResourceFromYaml(data []byte) (*api.Resource, error) {
	var resource api.Resource
	err := yaml.Unmarshal(data, &resource)
	if err != nil {
		return nil, fmt.Errorf("err: %v\n", err)
	}
	return &resource, nil
}

func LoadResourceFromYamlFile(file string) (*api.Resource, error) {
	var bytes []byte
	if file == "-" {
		bytes = make([]byte, 1024*10)
		read, err := io.ReadFull(os.Stdin, bytes)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "read %d byytes\n", read)
	} else {
		raw, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, err
		}
		bytes = raw
	}
	resource, err := LoadResourceFromYaml(bytes)
	if err != nil {
		return nil, err
	}
	resource.NameHashKey = resource.Metadata.Name
	return resource, nil
}

func LoadConfigFromYaml(data []byte) (*api.Config, error) {
	var config api.Config
	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("err: %v\n", err)
	}
	if config.Spec.Source != "" {
		remoteConfig, err := loadRemoteConfig(config.Spec.Source, &config)
		if err != nil {
			return nil, err
		}
		config.Spec.CustomResourceDefinitions = append(config.Spec.CustomResourceDefinitions, remoteConfig.Spec.CustomResourceDefinitions...)
	}
	return &config, nil
}

func LoadConfigFromYamlFile(configFile string) (*api.Config, error) {
	raw, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	return LoadConfigFromYaml(raw)
}

type configLoaders map[string]ConfigLoader

var ConfigLoaders configLoaders

func (l configLoaders) Register(scheme string, loader ConfigLoader) {
	if _, exists := l[scheme]; exists {
		panic(fmt.Errorf(`duplicate config loader for scheme "%s" detected`, scheme))
	}
	l[scheme] = loader
}

type ConfigLoader func(path string, context *api.Config) (*api.Config, error)

func init() {
	ConfigLoaders = configLoaders{}
	ConfigLoaders.Register("file", func(path string, _ *api.Config) (*api.Config, error) {
		return LoadConfigFromYamlFile(path)
	})
}

func loadRemoteConfig(source string, context *api.Config) (*api.Config, error) {
	parts := strings.Split(source, "://")
	if len(parts) != 2 {
		return nil, fmt.Errorf(`invalid format of source "%s": it must be formatted "<scheme>://<path>"`, source)
	}
	scheme := parts[0]
	path := parts[1]
	loader, exists := ConfigLoaders[scheme]
	if !exists {
		return nil, fmt.Errorf(`unexpected scheme "%s" found in source "%s": it must be one of %v`, scheme, source, []string{"file", "dynamodb"})
	}
	return loader(path, context)
}

func LoadResourceFromJson(data []byte) (*api.Resource, error) {
	var resource api.Resource
	err := json.Unmarshal(data, &resource)
	if err != nil {
		return nil, fmt.Errorf("err: %v\n", err)
	}
	return &resource, nil
}
