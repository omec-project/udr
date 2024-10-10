// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package service

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/udr/consumer"
	"github.com/omec-project/udr/context"
	"github.com/omec-project/udr/datarepository"
	"github.com/omec-project/udr/factory"
	"github.com/omec-project/udr/logger"
	"github.com/omec-project/udr/metrics"
	"github.com/omec-project/udr/producer"
	"github.com/omec-project/udr/util"
	"github.com/omec-project/util/http2_util"
	utilLogger "github.com/omec-project/util/logger"
	"github.com/omec-project/util/path_util"
	"github.com/urfave/cli"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type UDR struct{}

type (
	// Config information.
	Config struct {
		udrcfg string
	}
)

var config Config

var udrCLi = []cli.Flag{
	cli.StringFlag{
		Name:  "free5gccfg",
		Usage: "common config file",
	},
	cli.StringFlag{
		Name:  "udrcfg",
		Usage: "config file",
	},
}

var initLog *zap.SugaredLogger

var (
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
)

func init() {
	initLog = logger.InitLog
}

func (*UDR) GetCliCmd() (flags []cli.Flag) {
	return udrCLi
}

func (udr *UDR) Initialize(c *cli.Context) error {
	config = Config{
		udrcfg: c.String("udrcfg"),
	}

	if config.udrcfg != "" {
		if err := factory.InitConfigFactory(config.udrcfg); err != nil {
			return err
		}
	} else {
		DefaultUdrConfigPath := path_util.Free5gcPath("free5gc/config/udrcfg.yaml")
		if err := factory.InitConfigFactory(DefaultUdrConfigPath); err != nil {
			return err
		}
	}

	udr.setLogLevel()

	if err := factory.CheckConfigVersion(); err != nil {
		return err
	}

	return nil
}

func (udr *UDR) setLogLevel() {
	if factory.UdrConfig.Logger == nil {
		initLog.Warnln("UDR config without log level setting!!!")
		return
	}

	if factory.UdrConfig.Logger.UDR != nil {
		if factory.UdrConfig.Logger.UDR.DebugLevel != "" {
			if level, err := zapcore.ParseLevel(factory.UdrConfig.Logger.UDR.DebugLevel); err != nil {
				initLog.Warnf("UDR Log level [%s] is invalid, set to [info] level",
					factory.UdrConfig.Logger.UDR.DebugLevel)
				logger.SetLogLevel(zap.InfoLevel)
			} else {
				initLog.Infof("UDR Log level is set to [%s] level", level)
				logger.SetLogLevel(level)
			}
		} else {
			initLog.Infoln("UDR Log level not set. Default set to [info] level")
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

func (udr *UDR) FilterCli(c *cli.Context) (args []string) {
	for _, flag := range udr.GetCliCmd() {
		name := flag.GetName()
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
	initLog.Infof("udr config info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)

	// Connect to MongoDB
	producer.ConnectMongo(mongodb.Url, mongodb.Name, mongodb.AuthUrl, mongodb.AuthKeysDbName)
	initLog.Infoln("server started")

	router := utilLogger.NewGinWithZap(logger.GinLog)

	datarepository.AddService(router)

	go metrics.InitMetrics()

	udrLogPath := util.UdrLogPath

	self := context.UDR_Self()
	util.InitUdrContext(self)

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		udr.Terminate()
		os.Exit(0)
	}()

	go udr.registerNF()
	go udr.configUpdateDb()

	server, err := http2_util.NewServer(addr, udrLogPath, router)
	if server == nil {
		initLog.Errorf("initialize HTTP server failed: %+v", err)
		return
	}

	if err != nil {
		initLog.Warnf("initialize HTTP server: %+v", err)
	}

	serverScheme := factory.UdrConfig.Configuration.Sbi.Scheme
	if serverScheme == "http" {
		err = server.ListenAndServe()
	} else if serverScheme == "https" {
		err = server.ListenAndServeTLS(self.PEM, self.Key)
	}

	if err != nil {
		initLog.Fatalf("http server setup failed: %+v", err)
	}
}

func (udr *UDR) Exec(c *cli.Context) error {
	// UDR.Initialize(cfgPath, c)
	initLog.Debugln("args:", c.String("udrcfg"))
	args := udr.FilterCli(c)
	initLog.Debugln("filter:", args)
	command := exec.Command("./udr", args...)

	if err := udr.Initialize(c); err != nil {
		return err
	}

	var stdout io.ReadCloser
	if readCloser, err := command.StdoutPipe(); err != nil {
		initLog.Fatalln(err)
	} else {
		stdout = readCloser
	}
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		in := bufio.NewScanner(stdout)
		for in.Scan() {
			initLog.Debugln(in.Text())
		}
		wg.Done()
	}()

	var stderr io.ReadCloser
	if readCloser, err := command.StderrPipe(); err != nil {
		initLog.Fatalln(err)
	} else {
		stderr = readCloser
	}
	go func() {
		in := bufio.NewScanner(stderr)
		for in.Scan() {
			initLog.Debugln(in.Text())
		}
		wg.Done()
	}()

	var err error
	go func() {
		if errormessage := command.Start(); err != nil {
			initLog.Errorln("command.Start Failed")
			err = errormessage
		}
		wg.Done()
	}()

	wg.Wait()
	return err
}

func (udr *UDR) Terminate() {
	logger.InitLog.Infof("terminating UDR...")
	// deregister with NRF
	problemDetails, err := consumer.SendDeregisterNFInstance()
	if problemDetails != nil {
		logger.InitLog.Errorf("deregister NF instance Failed Problem[%+v]", problemDetails)
	} else if err != nil {
		logger.InitLog.Errorf("deregister NF instance Error[%+v]", err)
	} else {
		logger.InitLog.Infof("deregister from NRF successfully")
	}
	logger.InitLog.Infof("udr terminated")
}

func (udr *UDR) configUpdateDb() {
	for msg := range factory.ConfigUpdateDbTrigger {
		initLog.Infof("config update DB trigger")
		err := producer.AddEntrySmPolicyTable(
			msg.SmPolicyTable.Imsi,
			msg.SmPolicyTable.Dnn,
			msg.SmPolicyTable.Snssai)
		if err == nil {
			initLog.Infof("added entry to sm policy table success")
		} else {
			initLog.Errorf("entry add failed %+v", err)
		}
	}
}

func (udr *UDR) StartKeepAliveTimer(nfProfile models.NfProfile) {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	udr.StopKeepAliveTimer()
	if nfProfile.HeartBeatTimer == 0 {
		nfProfile.HeartBeatTimer = 60
	}
	logger.InitLog.Infof("started KeepAlive Timer: %v sec", nfProfile.HeartBeatTimer)
	// AfterFunc starts timer and waits for KeepAliveTimer to elapse and then calls udr.UpdateNF function
	KeepAliveTimer = time.AfterFunc(time.Duration(nfProfile.HeartBeatTimer)*time.Second, udr.UpdateNF)
}

func (udr *UDR) StopKeepAliveTimer() {
	if KeepAliveTimer != nil {
		logger.InitLog.Infof("stopped KeepAlive Timer.")
		KeepAliveTimer.Stop()
		KeepAliveTimer = nil
	}
}

func (udr *UDR) BuildAndSendRegisterNFInstance() (prof models.NfProfile, err error) {
	self := context.UDR_Self()
	profile := consumer.BuildNFInstance(self)
	initLog.Infof("udr Profile Registering to NRF: %v", profile)
	// Indefinite attempt to register until success
	profile, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile)
	return profile, err
}

// UpdateNF is the callback function, this is called when keepalivetimer elapsed
func (udr *UDR) UpdateNF() {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	if KeepAliveTimer == nil {
		initLog.Warnf("keepAlive timer has been stopped.")
		return
	}
	// setting default value 30 sec
	var heartBeatTimer int32 = 60
	pitem := models.PatchItem{
		Op:    "replace",
		Path:  "/nfStatus",
		Value: "REGISTERED",
	}
	var patchItem []models.PatchItem
	patchItem = append(patchItem, pitem)
	nfProfile, problemDetails, err := consumer.SendUpdateNFInstance(patchItem)
	if problemDetails != nil {
		initLog.Errorf("UDR update to NRF ProblemDetails[%v]", problemDetails)
		// 5xx response from NRF, 404 Not Found, 400 Bad Request
		if (problemDetails.Status/100) == 5 ||
			problemDetails.Status == 404 || problemDetails.Status == 400 {
			// register with NRF full profile
			nfProfile, err = udr.BuildAndSendRegisterNFInstance()
			if err != nil {
				initLog.Errorf("UDR register to NRF Error[%s]", err.Error())
			}
		}
	} else if err != nil {
		initLog.Errorf("UDR update to NRF Error[%s]", err.Error())
		nfProfile, err = udr.BuildAndSendRegisterNFInstance()
		if err != nil {
			initLog.Errorf("UDR register to NRF Error[%s]", err.Error())
		}
	}

	if nfProfile.HeartBeatTimer != 0 {
		// use hearbeattimer value with received timer value from NRF
		heartBeatTimer = nfProfile.HeartBeatTimer
	}
	logger.InitLog.Debugf("restarted KeepAlive Timer: %v sec", heartBeatTimer)
	// restart timer with received HeartBeatTimer value
	KeepAliveTimer = time.AfterFunc(time.Duration(heartBeatTimer)*time.Second, udr.UpdateNF)
}

func (udr *UDR) registerNF() {
	for msg := range factory.ConfigPodTrigger {
		initLog.Infof("minimum configuration from config pod available %v", msg)
		self := context.UDR_Self()
		profile := consumer.BuildNFInstance(self)
		var err error
		var prof models.NfProfile
		// send registration with updated PLMN Ids.
		prof, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, profile.NfInstanceId, profile)
		if err == nil {
			udr.StartKeepAliveTimer(prof)
			logger.CfgLog.Infoln("sent Register NF Instance with updated profile")
		} else {
			initLog.Errorf("send Register NFInstance Error[%s]", err.Error())
		}
	}
}
