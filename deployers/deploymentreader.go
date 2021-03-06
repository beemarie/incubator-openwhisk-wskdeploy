/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package deployers

import (
	"errors"
	"github.com/apache/incubator-openwhisk-client-go/whisk"
	"github.com/apache/incubator-openwhisk-wskdeploy/parsers"
	"github.com/apache/incubator-openwhisk-wskdeploy/wski18n"
	"github.com/apache/incubator-openwhisk-wskdeploy/wskderrors"
	"github.com/apache/incubator-openwhisk-wskdeploy/wskprint"
	"github.com/apache/incubator-openwhisk-wskdeploy/wskenv"
)

type DeploymentReader struct {
	serviceDeployer      *ServiceDeployer
	DeploymentDescriptor *parsers.YAML
}

func NewDeploymentReader(serviceDeployer *ServiceDeployer) *DeploymentReader {
	var dep DeploymentReader
	dep.serviceDeployer = serviceDeployer

	return &dep
}

// Wrapper parser to handle yaml dir
func (reader *DeploymentReader) HandleYaml() error {

	dep := reader.serviceDeployer

	deploymentParser := parsers.NewYAMLParser()
	deployment, err := deploymentParser.ParseDeployment(dep.DeploymentPath)

	reader.DeploymentDescriptor = deployment

	return err
}

// Update entities with deployment settings
func (reader *DeploymentReader) BindAssets() error {

	if err := reader.bindPackageInputsAndAnnotations(); err != nil {
		return err
	}
	if err := reader.bindActionInputsAndAnnotations(); err != nil {
		return err
	}
	if err := reader.bindTriggerInputsAndAnnotations(); err != nil {
		return err
	}

	return nil
}

func (reader *DeploymentReader) bindPackageInputsAndAnnotations() error {

	packMap := make(map[string]parsers.Package)

	if reader.DeploymentDescriptor.GetProject().Packages == nil {
		// a single package is specified in deployment YAML file with "package" key
		if len(reader.DeploymentDescriptor.GetProject().Package.Packagename) != 0 {
			packMap[reader.DeploymentDescriptor.GetProject().Package.Packagename] = reader.DeploymentDescriptor.GetProject().Package
			// TODO() i18n
			wskprint.PrintlnOpenWhiskWarning("The package YAML key in deployment file will soon be deprecated. Please use packages instead as described in specifications.")
		} else {
			if reader.DeploymentDescriptor.Packages != nil {
				for packName, depPacks := range reader.DeploymentDescriptor.Packages {
					depPacks.Packagename = packName
					packMap[packName] = depPacks
				}
			} else {
				packMap[reader.DeploymentDescriptor.Package.Packagename] = reader.DeploymentDescriptor.Package
			}
		}
	} else {
		for packName, depPacks := range reader.DeploymentDescriptor.GetProject().Packages {
			depPacks.Packagename = packName
			packMap[packName] = depPacks
		}
	}

	for packName, pack := range packMap {

		serviceDeployPack := reader.serviceDeployer.Deployment.Packages[packName]

		if serviceDeployPack == nil {
			// TODO() i18n
			wskprint.PrintlnOpenWhiskWarning("Package name in deployment file " + packName + " does not match with manifest file.")
			break
		}

		keyValArr := make(whisk.KeyValueArr, 0)

		if len(pack.Inputs) > 0 {
			for name, input := range pack.Inputs {
				var keyVal whisk.KeyValue

				keyVal.Key = name

				keyVal.Value = wskenv.GetEnvVar(input.Value)

				keyValArr = append(keyValArr, keyVal)
			}

			depParams := make(map[string]whisk.KeyValue)
			for _, kv := range keyValArr {
				depParams[kv.Key] = kv
			}

			for _, keyVal := range serviceDeployPack.Package.Parameters {
				if _, exists := depParams[keyVal.Key]; !exists {
					keyValArr = append(keyValArr, keyVal)
				}
			}

			serviceDeployPack.Package.Parameters = keyValArr
		}

		if len(pack.Annotations) > 0 {
			// iterate over each annotation from deployment file
			for name, input := range pack.Annotations {
				// check if annotation key in deployment file exists in manifest file
				// setting a bool flag to false assuming key does not exist in manifest
				keyExistsInManifest := false
				// iterate over each annotation from manifest file
				for i, a := range serviceDeployPack.Package.Annotations {
					if name == a.Key {
						// annotation key is found in manifest
						keyExistsInManifest = true
						// overwrite annotation in manifest file with deployment file
						serviceDeployPack.Package.Annotations[i].Value = input
						break
					}
				}
				if !keyExistsInManifest {
					// TODO() i18n, need to use an ID
					// TODO() fix grammar error; need command before "but"
					err := errors.New(wski18n.T("Annotation key \"" + name + "\" does not exist in manifest file but specified in deployment file."))
					return wskderrors.NewYAMLFileFormatError(reader.DeploymentDescriptor.Filepath, err)
				}
			}
		}
	}
	return nil
}

func (reader *DeploymentReader) bindActionInputsAndAnnotations() error {

	packMap := make(map[string]parsers.Package)

	if reader.DeploymentDescriptor.GetProject().Packages == nil {
		// a single package is specified in deployment YAML file with "package" key
		if len(reader.DeploymentDescriptor.GetProject().Package.Packagename) != 0 {
			packMap[reader.DeploymentDescriptor.GetProject().Package.Packagename] = reader.DeploymentDescriptor.GetProject().Package
		} else {
			if reader.DeploymentDescriptor.Packages != nil {
				for packName, depPacks := range reader.DeploymentDescriptor.Packages {
					depPacks.Packagename = packName
					packMap[packName] = depPacks
				}
			} else {
				packMap[reader.DeploymentDescriptor.Package.Packagename] = reader.DeploymentDescriptor.Package
			}
		}
	} else {
		for packName, depPacks := range reader.DeploymentDescriptor.GetProject().Packages {
			depPacks.Packagename = packName
			packMap[packName] = depPacks
		}
	}

	for packName, pack := range packMap {

		serviceDeployPack := reader.serviceDeployer.Deployment.Packages[packName]

		if serviceDeployPack == nil {
			break
		}

		for actionName, action := range pack.Actions {

			keyValArr := make(whisk.KeyValueArr, 0)

			if len(action.Inputs) > 0 {
				for name, input := range action.Inputs {
					var keyVal whisk.KeyValue

					keyVal.Key = name

					keyVal.Value = wskenv.GetEnvVar(input.Value)

					keyValArr = append(keyValArr, keyVal)
				}

				if wskAction, exists := serviceDeployPack.Actions[actionName]; exists {
					depParams := make(map[string]whisk.KeyValue)
					for _, kv := range keyValArr {
						depParams[kv.Key] = kv
					}

					for _, keyVal := range wskAction.Action.Parameters {
						if _, exists := depParams[keyVal.Key]; !exists {
							keyValArr = append(keyValArr, keyVal)
						}
					}
					wskAction.Action.Parameters = keyValArr
				}
			}

			if wskAction, exists := serviceDeployPack.Actions[actionName]; exists {
				// iterate over each annotation from deployment file
				for name, input := range action.Annotations {
					// check if annotation key in deployment file exists in manifest file
					// setting a bool flag to false assuming key does not exist in manifest
					keyExistsInManifest := false
					// iterate over each annotation from manifest file
					for i, a := range wskAction.Action.Annotations {
						if name == a.Key {
							// annotation key is found in manifest
							keyExistsInManifest = true
							// overwrite annotation in manifest file with deployment file
							wskAction.Action.Annotations[i].Value = input
							break
						}
					}
					if !keyExistsInManifest {
						// TODO() i18n, need to use an ID
						err := errors.New(wski18n.T("Annotation key \"" + name + "\" does not exist in manifest file but specified in deployment file."))
						return wskderrors.NewYAMLFileFormatError(reader.DeploymentDescriptor.Filepath, err)
					}
				}
			}
		}
	}
	return nil
}

func (reader *DeploymentReader) bindTriggerInputsAndAnnotations() error {

	packMap := make(map[string]parsers.Package)

	if reader.DeploymentDescriptor.GetProject().Packages == nil {
		if len(reader.DeploymentDescriptor.GetProject().Package.Packagename) != 0 {
			packMap[reader.DeploymentDescriptor.GetProject().Package.Packagename] = reader.DeploymentDescriptor.GetProject().Package
		} else {
			if reader.DeploymentDescriptor.Packages != nil {
				for packName, depPacks := range reader.DeploymentDescriptor.Packages {
					depPacks.Packagename = packName
					packMap[packName] = depPacks
				}
			} else {
				packMap[reader.DeploymentDescriptor.Package.Packagename] = reader.DeploymentDescriptor.Package
			}
		}
	} else {
		for packName, depPacks := range reader.DeploymentDescriptor.GetProject().Packages {
			depPacks.Packagename = packName
			packMap[packName] = depPacks
		}
	}

	for _, pack := range packMap {

		serviceDeployment := reader.serviceDeployer.Deployment

		for triggerName, trigger := range pack.Triggers {

			keyValArr := make(whisk.KeyValueArr, 0)

			if len(trigger.Inputs) > 0 {
				for name, input := range trigger.Inputs {
					var keyVal whisk.KeyValue

					keyVal.Key = name
					keyVal.Value = wskenv.GetEnvVar(input.Value)

					keyValArr = append(keyValArr, keyVal)
				}

				if wskTrigger, exists := serviceDeployment.Triggers[triggerName]; exists {

					depParams := make(map[string]whisk.KeyValue)
					for _, kv := range keyValArr {
						depParams[kv.Key] = kv
					}

					for _, keyVal := range wskTrigger.Parameters {
						// TODO() i18n
						wskprint.PrintlnOpenWhiskOutput("Checking key " + keyVal.Key)
						if _, exists := depParams[keyVal.Key]; !exists {
							keyValArr = append(keyValArr, keyVal)
						}
					}
					wskTrigger.Parameters = keyValArr
				}
			}

			if wskTrigger, exists := serviceDeployment.Triggers[triggerName]; exists {
				// iterate over each annotation from deployment file
				for name, input := range trigger.Annotations {
					// check if annotation key in deployment file exists in manifest file
					// setting a bool flag to false assuming key does not exist in manifest
					keyExistsInManifest := false
					// iterate over each annotation from manifest file
					for i, a := range wskTrigger.Annotations {
						if name == a.Key {
							// annotation key is found in manifest
							keyExistsInManifest = true
							// overwrite annotation in manifest file with deployment file
							wskTrigger.Annotations[i].Value = input
							break
						}
					}
					if !keyExistsInManifest {
						// TODO() i18n, need to use an ID
						err := errors.New(wski18n.T("Annotation key \"" + name + "\" does not exist in manifest file but specified in deployment file."))
						return wskderrors.NewYAMLFileFormatError(reader.DeploymentDescriptor.Filepath, err)
					}
				}
			}

		}

	}
	return nil
}
