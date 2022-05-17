// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

//+build debug

package util

import (
	"github.com/omec-project/path_util"
)

var (
	UdrLogPath           = path_util.Free5gcPath("omec-project/udrsslkey.log")
	UdrPemPath           = path_util.Free5gcPath("free5gc/support/TLS/_debug.pem")
	UdrKeyPath           = path_util.Free5gcPath("free5gc/support/TLS/_debug.key")
	DefaultUdrConfigPath = path_util.Free5gcPath("free5gc/config/udrcfg.yaml")
)
