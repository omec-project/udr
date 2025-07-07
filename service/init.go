// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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

	if factory.UdrConfig.Logger.MongoDBLibrary != nil {
		if factory.UdrConfig.Logger.MongoDBLibrary.DebugLevel != "" {
			if level, err := zapcore.ParseLevel(factory.UdrConfig.Logger.MongoDBLibrary.DebugLevel); err != nil {
				utilLogger.AppLog.Warnf("MongoDBLibrary Log level [%s] is invalid, set to [info] level",
					factory.UdrConfig.Logger.MongoDBLibrary.DebugLevel)
				utilLogger.SetLogLevel(zap.InfoLevel)
			} else {
				utilLogger.SetLogLevel(level)
			}
		} else {
			utilLogger.AppLog.Warnln("MongoDBLibrary Log level not set. Default set to [info] level")
			utilLogger.SetLogLevel(zap.InfoLevel)
		}
	}
}

func (udr *UDR) FilterCli(c *cli.Command) (args []string) {
	for _, flag := range udr.GetCliCmd() {
		name := flag.Names()[0]
		value := fmt.Sprint(c.Generic(name))
		if value == "" {
			continue
		}

		args = append(args, "--"+name, value)
	}
	return args
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

func (udr *UDR) Exec(c *cli.Command) error {
	// UDR.Initialize(cfgPath, c)
	logger.InitLog.Debugln("args:", c.String("cfg"))
	args := udr.FilterCli(c)
	logger.InitLog.Debugln("filter:", args)
	command := exec.Command("udr", args...)

	if err := udr.Initialize(c); err != nil {
		return err
	}

	var stdout io.ReadCloser
	if readCloser, err := command.StdoutPipe(); err != nil {
		logger.InitLog.Fatalln(err)
	} else {
		stdout = readCloser
	}
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		in := bufio.NewScanner(stdout)
		for in.Scan() {
			logger.InitLog.Debugln(in.Text())
		}
		wg.Done()
	}()

	var stderr io.ReadCloser
	if readCloser, err := command.StderrPipe(); err != nil {
		logger.InitLog.Fatalln(err)
	} else {
		stderr = readCloser
	}
	go func() {
		in := bufio.NewScanner(stderr)
		for in.Scan() {
			logger.InitLog.Debugln(in.Text())
		}
		wg.Done()
	}()

	var err error
	go func() {
		if errormessage := command.Start(); err != nil {
			logger.InitLog.Errorln("command.Start Failed")
			err = errormessage
		}
		wg.Done()
	}()

	wg.Wait()
	return err
}

func (udr *UDR) Terminate(cancelServices context.CancelFunc, wg *sync.WaitGroup) {
	logger.InitLog.Infoln("terminating UDR")
	cancelServices()
	nfregistration.DeregisterNF()
	wg.Wait()
	logger.InitLog.Infoln("UDR terminated")
}
