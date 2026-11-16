// Copyright (C) 2026 Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package factory

import (
	"github.com/omec-project/udr/logger"
	"github.com/omec-project/util/nfconfig"
)

var UdrConfig Config

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) error {
	UdrConfig = Config{}

	if err := nfconfig.LoadYAML(f, &UdrConfig); err != nil {
		return err
	}
	if UdrConfig.Configuration.Mongodb.AuthUrl == "" {
		authUrl := UdrConfig.Configuration.Mongodb.Url
		UdrConfig.Configuration.Mongodb.AuthUrl = authUrl
	}
	if UdrConfig.Configuration.Mongodb.AuthKeysDbName == "" {
		UdrConfig.Configuration.Mongodb.AuthKeysDbName = "authentication"
	}

	webuiUri, err := nfconfig.ResolveWebuiUri(UdrConfig.Configuration.WebuiUri, logger.CfgLog)
	if err != nil {
		return err
	}
	UdrConfig.Configuration.WebuiUri = webuiUri
	return nil
}

func CheckConfigVersion() error {
	return nfconfig.CheckVersion(UdrConfig.GetVersion(), UDR_EXPECTED_CONFIG_VERSION, logger.CfgLog)
}
