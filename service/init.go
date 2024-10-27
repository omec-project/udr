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
	"path/filepath"
	"sync"
	"syscall"
	"time"

	grpcClient "github.com/omec-project/config5g/proto/client"
	protos "github.com/omec-project/config5g/proto/sdcoreConfig"
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
	"github.com/urfave/cli"
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
	cli.StringFlag{
		Name:     "cfg",
		Usage:    "udr config file",
		Required: true,
	},
}

var (
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
)

func (*UDR) GetCliCmd() (flags []cli.Flag) {
	return udrCLi
}

func (udr *UDR) Initialize(c *cli.Context) error {
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

	if os.Getenv("MANAGED_BY_CONFIG_POD") == "true" {
		logger.InitLog.Infoln("MANAGED_BY_CONFIG_POD is true")
		go manageGrpcClient(factory.UdrConfig.Configuration.WebuiUri)
	} else {
		go func() {
			logger.InitLog.Infoln("use helm chart config")
			factory.ConfigPodTrigger <- true
		}()
	}

	return nil
}

// manageGrpcClient connects the config pod GRPC server and subscribes the config changes.
// Then it updates UDR configuration.
func manageGrpcClient(webuiUri string) {
	var configChannel chan *protos.NetworkSliceResponse
	var client grpcClient.ConfClient
	var stream protos.ConfigService_NetworkSliceSubscribeClient
	var err error
	count := 0
	for {
		if client != nil {
			if client.CheckGrpcConnectivity() != "ready" {
				time.Sleep(time.Second * 30)
				count++
				if count > 5 {
					err = client.GetConfigClientConn().Close()
					if err != nil {
						logger.InitLog.Infof("failing ConfigClient is not closed properly: %+v", err)
					}
					client = nil
					count = 0
				}
				logger.InitLog.Infoln("checking the connectivity readiness")
				continue
			}

			if stream == nil {
				stream, err = client.SubscribeToConfigServer()
				if err != nil {
					logger.InitLog.Infof("failing SubscribeToConfigServer: %+v", err)
					continue
				}
			}

			if configChannel == nil {
				configChannel = client.PublishOnConfigChange(true, stream)
				logger.InitLog.Infoln("PublishOnConfigChange is triggered")
				factory.ConfigUpdateDbTrigger = make(chan *factory.UpdateDb, 10)
				go factory.UdrConfig.UpdateConfig(configChannel, factory.ConfigUpdateDbTrigger)
				logger.InitLog.Infoln("UDR updateConfig is triggered")
			}
		} else {
			client, err = grpcClient.ConnectToConfigServer(webuiUri)
			stream = nil
			configChannel = nil
			logger.InitLog.Infoln("connecting to config server")
			if err != nil {
				logger.InitLog.Errorf("%+v", err)
			}
			continue
		}
	}
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
	logger.InitLog.Infof("udr config info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)

	// Connect to MongoDB
	producer.ConnectMongo(mongodb.Url, mongodb.Name, mongodb.AuthUrl, mongodb.AuthKeysDbName)
	logger.InitLog.Infoln("server started")

	router := utilLogger.NewGinWithZap(logger.GinLog)

	datarepository.AddService(router)

	go metrics.InitMetrics()

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
	if serverScheme == "http" {
		err = server.ListenAndServe()
	} else if serverScheme == "https" {
		err = server.ListenAndServeTLS(self.PEM, self.Key)
	}

	if err != nil {
		logger.InitLog.Fatalf("http server setup failed: %+v", err)
	}
}

func (udr *UDR) Exec(c *cli.Context) error {
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

func (udr *UDR) Terminate() {
	logger.InitLog.Infoln("terminating UDR")
	// deregister with NRF
	problemDetails, err := consumer.SendDeregisterNFInstance()
	if problemDetails != nil {
		logger.InitLog.Errorf("deregister NF instance Failed Problem[%+v]", problemDetails)
	} else if err != nil {
		logger.InitLog.Errorf("deregister NF instance Error[%+v]", err)
	} else {
		logger.InitLog.Infoln("deregister from NRF successfully")
	}
	logger.InitLog.Infoln("UDR terminated")
}

func (udr *UDR) configUpdateDb() {
	for msg := range factory.ConfigUpdateDbTrigger {
		logger.InitLog.Infoln("config update DB trigger")
		err := producer.AddEntrySmPolicyTable(
			msg.SmPolicyTable.Imsi,
			msg.SmPolicyTable.Dnn,
			msg.SmPolicyTable.Snssai)
		if err == nil {
			logger.InitLog.Infoln("added entry to sm policy table success")
		} else {
			logger.InitLog.Errorf("entry add failed %+v", err)
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
	logger.InitLog.Infof("started KeepAlive timer: %v sec", nfProfile.HeartBeatTimer)
	// AfterFunc starts timer and waits for KeepAliveTimer to elapse and then calls udr.UpdateNF function
	KeepAliveTimer = time.AfterFunc(time.Duration(nfProfile.HeartBeatTimer)*time.Second, udr.UpdateNF)
}

func (udr *UDR) StopKeepAliveTimer() {
	if KeepAliveTimer != nil {
		logger.InitLog.Infoln("stopped KeepAlive timer")
		KeepAliveTimer.Stop()
		KeepAliveTimer = nil
	}
}

func (udr *UDR) BuildAndSendRegisterNFInstance() (prof models.NfProfile, err error) {
	self := context.UDR_Self()
	profile := consumer.BuildNFInstance(self)
	logger.InitLog.Infof("UDR profile registering to NRF: %v", profile)
	// Indefinite attempt to register until success
	profile, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile)
	return profile, err
}

// UpdateNF is the callback function, this is called when keepalivetimer elapsed
func (udr *UDR) UpdateNF() {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	if KeepAliveTimer == nil {
		logger.InitLog.Warnln("keepAlive timer has been stopped")
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
		logger.InitLog.Errorf("UDR update to NRF ProblemDetails[%v]", problemDetails)
		// 5xx response from NRF, 404 Not Found, 400 Bad Request
		if (problemDetails.Status/100) == 5 ||
			problemDetails.Status == 404 || problemDetails.Status == 400 {
			// register with NRF full profile
			nfProfile, err = udr.BuildAndSendRegisterNFInstance()
			if err != nil {
				logger.InitLog.Errorf("UDR register to NRF Error[%s]", err.Error())
			}
		}
	} else if err != nil {
		logger.InitLog.Errorf("UDR update to NRF Error[%s]", err.Error())
		nfProfile, err = udr.BuildAndSendRegisterNFInstance()
		if err != nil {
			logger.InitLog.Errorf("UDR register to NRF Error[%s]", err.Error())
		}
	}

	if nfProfile.HeartBeatTimer != 0 {
		// use hearbeattimer value with received timer value from NRF
		heartBeatTimer = nfProfile.HeartBeatTimer
	}
	logger.InitLog.Debugf("restarted KeepAlive timer: %v sec", heartBeatTimer)
	// restart timer with received HeartBeatTimer value
	KeepAliveTimer = time.AfterFunc(time.Duration(heartBeatTimer)*time.Second, udr.UpdateNF)
}

func (udr *UDR) registerNF() {
	for msg := range factory.ConfigPodTrigger {
		logger.InitLog.Infof("minimum configuration from config pod available %v", msg)
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
			logger.InitLog.Errorf("send Register NFInstance Error[%s]", err.Error())
		}
	}
}
