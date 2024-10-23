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
	"time"

	grpcClient "github.com/omec-project/config5g/proto/client"
	protos "github.com/omec-project/config5g/proto/sdcoreConfig"
	"github.com/omec-project/udr/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc/connectivity"
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

var initLog *zap.SugaredLogger

func init() {
	initLog = logger.InitLog
}

// InitConfigFactory gets the UdrConfig and subscribes the config pod.
// This observes the GRPC client availability and connection status in a loop.
// When the GRPC server pod is restarted, GRPC connection status stuck in idle.
// If GRPC client does not exist, creates it. If client exists but GRPC connectivity is not ready,
// then it closes the existing client start a new client.
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
			initLog.Infoln("MANAGED_BY_CONFIG_POD is true")
			client, err := grpcClient.ConnectToConfigServer(UdrConfig.Configuration.WebuiUri)
			if err != nil {
				logger.InitLog.Infof("Connect to config server failed: %v", err)
			}
			go manageGrpcClient(client)
		} else {
			go func() {
				initLog.Infoln("Use helm chart config ")
				ConfigPodTrigger <- true
			}()
		}
	}

	return nil
}

// manageGrpcClient connects the config pod GRPC server and subscribes the config changes
// then updates UDR configuration
func manageGrpcClient(client grpcClient.ConfClient) {
	var stream protos.ConfigService_NetworkSliceSubscribeClient
	var err error
	var configChannel chan *protos.NetworkSliceResponse
	for {
		if client != nil {
			stream, err = client.CheckGrpcConnectivity()
			if err != nil {
				logger.InitLog.Errorf("%v", err)
			}
			if stream == nil {
				time.Sleep(time.Second * 30)
				continue
			}
			time.Sleep(time.Second * 30)
			if client.GetConfigClientConn().GetState() != connectivity.Ready {
				err = client.GetConfigClientConn().Close()
				if err != nil {
					logger.InitLog.Debugf("failing ConfigClient is not closed properly: %+v", err)
				}
				client = nil
				continue
			}
			if configChannel == nil {
				configChannel = client.PublishOnConfigChange(true, stream)
				ConfigUpdateDbTrigger = make(chan *UpdateDb, 10)
				go UdrConfig.updateConfig(configChannel, ConfigUpdateDbTrigger)
			}

		} else {
			client, err = grpcClient.ConnectToConfigServer(UdrConfig.Configuration.WebuiUri)
			if err != nil {
				logger.InitLog.Errorf("%+v", err)
			}
			continue
		}
	}
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
