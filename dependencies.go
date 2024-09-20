// In gitspace/dependencies.go
package main

import (
	"encoding/json"
	"os/exec"
	"bytes"
)

func GetCurrentDependencies() (map[string]string, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var modules []struct {
		Path    string
		Version string
	}

	decoder := json.NewDecoder(bytes.NewReader(output))
	for decoder.More() {
		var module struct {
			Path    string
			Version string
		}
		if err := decoder.Decode(&module); err != nil {
			return nil, err
		}
		modules = append(modules, module)
	}

	dependencies := make(map[string]string)
	for _, module := range modules {
		if module.Path != "github.com/ssotops/gitspace" { // Exclude the main module
			dependencies[module.Path] = module.Version
		}
	}

	return dependencies, nil
}
