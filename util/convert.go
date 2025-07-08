// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package util

import (
	"encoding/json"
	"fmt"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/udr/logger"
	"go.mongodb.org/mongo-driver/bson"
)

func MapToByte(data map[string]interface{}) []byte {
	ret, err := json.Marshal(data)
	if err != nil {
		logger.UtilLog.Error(err)
	}
	return ret
}

func MapArrayToByte(data []map[string]interface{}) []byte {
	ret, err := json.Marshal(data)
	if err != nil {
		logger.UtilLog.Error(err)
	}
	return ret
}

func PrimitiveAToByte(data []interface{}) []byte {
	ret, err := json.Marshal(data)
	if err != nil {
		logger.UtilLog.Error(err)
	}
	return ret
}

func ToBsonM(data interface{}) bson.M {
	tmp, err := json.Marshal(data)
	if err != nil {
		logger.UtilLog.Error(err)
	}
	putData := bson.M{}
	err = json.Unmarshal(tmp, &putData)
	if err != nil {
		logger.UtilLog.Error(err)
	}
	return putData
}

func SnssaiModelsToHex(snssai models.Snssai) string {
	sst := fmt.Sprintf("%02x", snssai.Sst)
	return sst + snssai.Sd
}
