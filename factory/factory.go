/*
 * UDR Configuration Factory
 */

package factory

import (
	"fmt"
	"io/ioutil"
	"reflect"

	"gopkg.in/yaml.v2"

	"github.com/free5gc/udr/logger"
)

var UdrConfig Config

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) error {
	if content, err := ioutil.ReadFile(f); err != nil {
		return err
	} else {
		UdrConfig = Config{}

		if yamlErr := yaml.Unmarshal(content, &UdrConfig); yamlErr != nil {
			return yamlErr
		}
	}

	return nil
}

func UpdateUdrConfig(f string) error {
	if content, err := ioutil.ReadFile(f); err != nil {
		return err
	} else {
		var udrConfig Config

		if yamlErr := yaml.Unmarshal(content, &udrConfig); yamlErr != nil {
			return yamlErr
		}
		//Checking which config has been changed
		if reflect.DeepEqual(UdrConfig.Configuration.NrfUri, udrConfig.Configuration.NrfUri) == false {
			logger.CfgLog.Infoln("updated NrfUri ", udrConfig.Configuration.NrfUri)
		}
		if reflect.DeepEqual(udrConfig.Configuration.Sbi, UdrConfig.Configuration.Sbi) == false {
			logger.CfgLog.Infoln("updated Sbi ", udrConfig.Configuration.Sbi)
		}
		if reflect.DeepEqual(udrConfig.Configuration.Mongodb, UdrConfig.Configuration.Mongodb) == false {
			logger.CfgLog.Infoln("updated Mongodb ", udrConfig.Configuration.Mongodb)
		}
		UdrConfig = udrConfig
	}

	return nil
}

func CheckConfigVersion() error {
	currentVersion := UdrConfig.GetVersion()

	if currentVersion != UDR_EXPECTED_CONFIG_VERSION {
		return fmt.Errorf("config version is [%s], but expected is [%s].",
			currentVersion, UDR_EXPECTED_CONFIG_VERSION)
	}

	logger.CfgLog.Infof("config version [%s]", currentVersion)

	return nil
}
