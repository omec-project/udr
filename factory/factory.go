// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

/*
 * UDR Configuration Factory
 */

package factory

import (
	"fmt"
	"os"

	protos "github.com/omec-project/config5g/proto/sdcoreConfig"
	"github.com/omec-project/udr/logger"
<<<<<<< HEAD
=======
	"go.uber.org/zap"
	"google.golang.org/grpc/connectivity"
>>>>>>> 4fa3dfc (fix: rename and organize a method)
	"gopkg.in/yaml.v2"
)

var UdrConfig Config

type UpdateDb struct {
	SmPolicyTable *SmPolicyUpdateEntry
}

type SmPolicyUpdateEntry struct {
	Snssai *protos.NSSAI
	Imsi   string
	Dnn    string
}

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) error {
	if content, err := os.ReadFile(f); err != nil {
		return err
	} else {
		UdrConfig = Config{}

		if yamlErr := yaml.Unmarshal(content, &UdrConfig); yamlErr != nil {
			return yamlErr
		}
		if UdrConfig.Configuration.Mongodb.AuthUrl == "" {
			authUrl := UdrConfig.Configuration.Mongodb.Url
			UdrConfig.Configuration.Mongodb.AuthUrl = authUrl
		}
		if UdrConfig.Configuration.Mongodb.AuthKeysDbName == "" {
			UdrConfig.Configuration.Mongodb.AuthKeysDbName = "authentication"
		}
		if UdrConfig.Configuration.WebuiUri == "" {
			UdrConfig.Configuration.WebuiUri = "webui:9876"
		}
		if os.Getenv("MANAGED_BY_CONFIG_POD") == "true" {
			logger.InitLog.Infoln("MANAGED_BY_CONFIG_POD is true")
		} else {
			go func() {
				logger.InitLog.Infoln("use helm chart config")
				ConfigPodTrigger <- true
			}()
		}
	}

	return nil
}

func CheckConfigVersion() error {
	currentVersion := UdrConfig.GetVersion()

	if currentVersion != UDR_EXPECTED_CONFIG_VERSION {
		return fmt.Errorf("config version is [%s], but expected is [%s]",
			currentVersion, UDR_EXPECTED_CONFIG_VERSION)
	}

	logger.CfgLog.Infof("config version [%s]", currentVersion)

	return nil
}
