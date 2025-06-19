// Copyright 2024-present Intel Corporation
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/omec-project/udr/logger"
	"github.com/omec-project/udr/service"
	"github.com/urfave/cli/v3"
)

var UDR = &service.UDR{}

func main() {
	app := &cli.Command{}
	app.Name = "udr"
	logger.AppLog.Infoln(app.Name)
	app.Usage = "Unified Data Repository"
	app.UsageText = "udr -cfg <udr_config_file.conf>"
	app.Action = action
	app.Flags = UDR.GetCliCmd()
	if err := app.Run(context.Background(), os.Args); err != nil {
		logger.AppLog.Fatalf("UDR run error: %v", err)
	}
}

func action(ctx context.Context, c *cli.Command) error {
	if err := UDR.Initialize(c); err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to initialize")
	}

	UDR.Start()

	return nil
}
