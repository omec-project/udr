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
<<<<<<< HEAD
<<<<<<< HEAD
	"google.golang.org/grpc/connectivity"
>>>>>>> 4fa3dfc (fix: rename and organize a method)
=======
>>>>>>> bd9df4e (fix: change method)
=======
	"google.golang.org/grpc/connectivity"
>>>>>>> 7bb8bf6 (modify subscribeToConfigPod)
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

<<<<<<< HEAD
=======
// manageGrpcClient connects the config pod GRPC server and subscribes the config changes.
// Then it updates UDR configuration.
func manageGrpcClient(webuiUri string) {
	var configChannel chan *protos.NetworkSliceResponse
	var client ConfClient
	var err error
	for {
		if client != nil {
			_, err = client.CheckGrpcConnectivity()
			if err != nil {
				initLog.Infoln("Connectivity error, waiting 30 seconds")
				time.Sleep(time.Second * 30)
			}
			time.Sleep(time.Second * 30)
			if client.GetConfigClientConn().GetState() != connectivity.Ready {
				err = client.GetConfigClientConn().Close()
				if err != nil {
					initLog.Infof("failing ConfigClient is not closed properly: %+v", err)
				}
				client = nil
				continue
			}
			if configChannel == nil {
				configChannel = client.PublishOnConfigChange(true)
				initLog.Infoln("PublishOnConfigChange is triggered.")
				ConfigUpdateDbTrigger = make(chan *UpdateDb, 10)
				go UdrConfig.updateConfig(configChannel, ConfigUpdateDbTrigger)
				initLog.Infoln("UDR updateConfig is triggered.")
			}
		} else {
			client, err = ConnectToConfigServer(webuiUri)
			initLog.Infoln("Connecting to config server.")
			if err != nil {
				logger.InitLog.Errorf("%+v", err)
			}
			continue
		}
	}
}

>>>>>>> 7bb8bf6 (modify subscribeToConfigPod)
func CheckConfigVersion() error {
	currentVersion := UdrConfig.GetVersion()

	if currentVersion != UDR_EXPECTED_CONFIG_VERSION {
		return fmt.Errorf("config version is [%s], but expected is [%s]",
			currentVersion, UDR_EXPECTED_CONFIG_VERSION)
	}

	logger.CfgLog.Infof("config version [%s]", currentVersion)

	return nil
}
