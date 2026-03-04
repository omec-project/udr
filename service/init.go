// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/omec-project/openapi/models"
	udrContext "github.com/omec-project/udr/context"
	"github.com/omec-project/udr/datarepository"
	"github.com/omec-project/udr/factory"
	"github.com/omec-project/udr/logger"
	"github.com/omec-project/udr/metrics"
	"github.com/omec-project/udr/nfregistration"
	"github.com/omec-project/udr/polling"
	"github.com/omec-project/udr/producer"
	"github.com/omec-project/udr/util"
	"github.com/omec-project/util/http2_util"
	utilLogger "github.com/omec-project/util/logger"
	"github.com/urfave/cli/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type UDR struct{}

type (
	// Config information.
	Config struct {
		cfg string
	}
)

var config Config

var udrCLi = []cli.Flag{
	&cli.StringFlag{
		Name:     "cfg",
		Usage:    "udr config file",
		Required: true,
	},
}

func (*UDR) GetCliCmd() (flags []cli.Flag) {
	return udrCLi
}

func (udr *UDR) Initialize(c *cli.Command) error {
	config = Config{
		cfg: c.String("cfg"),
	}

	absPath, err := filepath.Abs(config.cfg)
	if err != nil {
		logger.CfgLog.Errorln(err)
		return err
	}

	if err := factory.InitConfigFactory(absPath); err != nil {
		return err
	}

	factory.UdrConfig.CfgLocation = absPath

	udr.setLogLevel()

	if err := factory.CheckConfigVersion(); err != nil {
		return err
	}
	return nil
}

func (udr *UDR) setLogLevel() {
	if factory.UdrConfig.Logger == nil {
		logger.InitLog.Warnln("UDR config without log level setting")
		return
	}

	if factory.UdrConfig.Logger.UDR != nil {
		if factory.UdrConfig.Logger.UDR.DebugLevel != "" {
			if level, err := zapcore.ParseLevel(factory.UdrConfig.Logger.UDR.DebugLevel); err != nil {
				logger.InitLog.Warnf("UDR Log level [%s] is invalid, set to [info] level",
					factory.UdrConfig.Logger.UDR.DebugLevel)
				logger.SetLogLevel(zap.InfoLevel)
			} else {
				logger.InitLog.Infof("UDR Log level is set to [%s] level", level)
				logger.SetLogLevel(level)
			}
		} else {
			logger.InitLog.Infoln("UDR Log level not set. Default set to [info] level")
			logger.SetLogLevel(zap.InfoLevel)
		}
	}

	if factory.UdrConfig.Logger.Util != nil {
		if factory.UdrConfig.Logger.Util.DebugLevel != "" {
			if level, err := zapcore.ParseLevel(factory.UdrConfig.Logger.Util.DebugLevel); err != nil {
				utilLogger.UtilLog.Warnf("Util Log level [%s] is invalid, set to [info] level",
					factory.UdrConfig.Logger.Util.DebugLevel)
				utilLogger.SetLogLevel(zap.InfoLevel)
			} else {
				utilLogger.SetLogLevel(level)
			}
		} else {
			utilLogger.UtilLog.Warnln("Util Log level not set. Default set to [info] level")
			utilLogger.SetLogLevel(zap.InfoLevel)
		}
	}
}

func (udr *UDR) Start() {
	// get config file info
	config := factory.UdrConfig
	mongodb := config.Configuration.Mongodb
	logger.InitLog.Infof("udr config info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)

	// Connect to MongoDB
	producer.ConnectMongo(mongodb.Url, mongodb.Name, mongodb.AuthUrl, mongodb.AuthKeysDbName)
	logger.InitLog.Infoln("server started")

	router := utilLogger.NewGinWithZap(logger.GinLog)

	datarepository.AddService(router)

	go metrics.InitMetrics()

	self := udrContext.UDR_Self()
	util.InitUdrContext(self)

	plmnConfigChan := make(chan []models.PlmnId, 1)
	ctx, cancelServices := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		polling.StartPollingService(ctx, factory.UdrConfig.Configuration.WebuiUri, plmnConfigChan)
	}()
	go func() {
		defer wg.Done()
		nfregistration.StartNfRegistrationService(ctx, plmnConfigChan)
	}()

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		udr.Terminate(cancelServices, &wg)
		os.Exit(0)
	}()

	sslLog := filepath.Dir(factory.UdrConfig.CfgLocation) + "/sslkey.log"
	server, err := http2_util.NewServer(addr, sslLog, router)
	if server == nil {
		logger.InitLog.Errorf("initialize HTTP server failed: %+v", err)
		return
	}

	if err != nil {
		logger.InitLog.Warnf("initialize HTTP server: %+v", err)
	}

	serverScheme := factory.UdrConfig.Configuration.Sbi.Scheme
	switch serverScheme {
	case "http":
		err = server.ListenAndServe()
	case "https":
		err = server.ListenAndServeTLS(self.PEM, self.Key)
	default:
		logger.InitLog.Fatalf("HTTP server setup failed: invalid server scheme %+v", serverScheme)
		return
	}

	if err != nil {
		logger.InitLog.Fatalf("http server setup failed: %+v", err)
	}
}

func (udr *UDR) Terminate(cancelServices context.CancelFunc, wg *sync.WaitGroup) {
	logger.InitLog.Infoln("terminating UDR")
	cancelServices()
	nfregistration.DeregisterNF()
	wg.Wait()
	logger.InitLog.Infoln("UDR terminated")
}
