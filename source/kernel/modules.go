/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kernel

import (
	"fmt"
	"io/ioutil"
	"strings"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
)

const kmodProcfsPath = "/proc/modules"

func getLoadedModules() (map[string]feature.Nil, error) {
	out, err := ioutil.ReadFile(kmodProcfsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %s", kmodProcfsPath, err.Error())
	}

	loadedMods := make(map[string]feature.Nil)
	for _, line := range strings.Split(string(out), "\n") {
		// skip empty lines
		if len(line) == 0 {
			continue
		}
		// append loaded module
		loadedMods[strings.Fields(line)[0]] = feature.Nil{}
	}
	return loadedMods, nil
}
