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
	"net/url"
	"os"

	"github.com/omec-project/udr/logger"
	"go.yaml.in/yaml/v4"
)

var UdrConfig Config

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) error {
	content, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	UdrConfig = Config{}

	if err = yaml.Unmarshal(content, &UdrConfig); err != nil {
		return err
	}
	if UdrConfig.Configuration.Mongodb.AuthUrl == "" {
		authUrl := UdrConfig.Configuration.Mongodb.Url
		UdrConfig.Configuration.Mongodb.AuthUrl = authUrl
	}
	if UdrConfig.Configuration.Mongodb.AuthKeysDbName == "" {
		UdrConfig.Configuration.Mongodb.AuthKeysDbName = "authentication"
	}

	if UdrConfig.Configuration.WebuiUri == "" {
		UdrConfig.Configuration.WebuiUri = "http://webui:5001"
		logger.CfgLog.Infof("webuiUri not set in configuration file. Using %v", UdrConfig.Configuration.WebuiUri)
		return nil
	}
	err = validateWebuiUri(UdrConfig.Configuration.WebuiUri)
	return err
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

func validateWebuiUri(uri string) error {
	parsedUrl, err := url.ParseRequestURI(uri)
	if err != nil {
		return err
	}
	if parsedUrl.Scheme != "http" && parsedUrl.Scheme != "https" {
		return fmt.Errorf("unsupported scheme for webuiUri: %s", parsedUrl.Scheme)
	}
	if parsedUrl.Hostname() == "" {
		return fmt.Errorf("missing host in webuiUri")
	}
	return nil
}
