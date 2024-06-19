package main

import (
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	exampleConfigFile, err := os.ReadFile("config/example_config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	newConfigFile, err := os.ReadFile("config/dumped_config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// unmarshal ymal
	exampleConfig := make(map[string]interface{})
	if err := yaml.Unmarshal(exampleConfigFile, &exampleConfig); err != nil {
		log.Fatal(err)
	}
	exampleConfig = convertToLowercase(exampleConfig)

	newConfig := make(map[string]interface{})
	if err := yaml.Unmarshal(newConfigFile, &newConfig); err != nil {
		log.Fatal(err)
	}

	// iterate key in dumped config
	for new_key := range newConfig {
		// check whether key exists in example_config
		if _, ok := exampleConfig[new_key]; !ok {
			// return an error if not
			log.Fatalf("key: %s does not exist in /config/example_config.yaml", new_key)
		}
	}
}

func convertToLowercase(configMap map[string]interface{}) map[string]interface{} {
	lowercase := make(map[string]interface{}, len(configMap))
	for k, v := range configMap {
		lowercase[strings.ToLower(k)] = v
	}
	return lowercase
}
