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

	"gopkg.in/yaml.v2"

	"github.com/omec-project/config5g/proto/client"
	protos "github.com/omec-project/config5g/proto/sdcoreConfig"
	"github.com/omec-project/udr/logger"
	"github.com/sirupsen/logrus"
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

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
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
		roc := os.Getenv("MANAGED_BY_CONFIG_POD")
		if roc == "true" {
			initLog.Infoln("MANAGED_BY_CONFIG_POD is true")
			commChannel := client.ConfigWatcher()
			ConfigUpdateDbTrigger = make(chan *UpdateDb, 10)
			go UdrConfig.updateConfig(commChannel, ConfigUpdateDbTrigger)
		} else {
			go func() {
				initLog.Infoln("Use helm chart config ")
				ConfigPodTrigger <- true
			}()
		}
	}

	return nil
}

func CheckConfigVersion() error {
	currentVersion := UdrConfig.GetVersion()

	if currentVersion != UDR_EXPECTED_CONFIG_VERSION {
		return fmt.Errorf("config version is [%s], but expected is [%s].",
			currentVersion, UDR_EXPECTED_CONFIG_VERSION)
	}

	logger.CfgLog.Infof("config version [%s]", currentVersion)

	return nil
}
