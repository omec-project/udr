// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package consumer

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/omec-project/openapi/v2"
	"github.com/omec-project/openapi/v2/Nnrf_NFManagement"
	"github.com/omec-project/openapi/v2/models"
	udrContext "github.com/omec-project/udr/context"
	"github.com/omec-project/udr/factory"
	"github.com/omec-project/udr/logger"
)

func getNfProfile(udrContext *udrContext.UDRContext, plmnConfig []models.PlmnId) (*models.NFProfile, error) {
	if udrContext == nil {
		return &models.NFProfile{}, fmt.Errorf("udr context has not been intialized. NF profile cannot be built")
	}
	var profile models.NFProfile
	config := factory.UdrConfig
	profile.NfInstanceId = udrContext.NfId
	profile.NfType = models.NFTYPE_UDR
	profile.NfStatus = models.NFSTATUS_REGISTERED
	if len(plmnConfig) > 0 {
		plmnCopy := make([]models.PlmnId, len(plmnConfig))
		copy(plmnCopy, plmnConfig)
		profile.PlmnList = plmnCopy
	}

	version := config.Info.Version
	tmpVersion := strings.Split(version, ".")
	versionUri := "v" + tmpVersion[0]
	apiPrefix := fmt.Sprintf("%s://%s:%d", udrContext.UriScheme, udrContext.RegisterIPv4, udrContext.SBIPort)
	ipEndPoint := models.NewIpEndPoint()
	ipEndPoint.SetIpv4Address(udrContext.RegisterIPv4)
	ipEndPoint.SetTransport(models.TRANSPORTPROTOCOL_TCP)
	ipEndPoint.SetPort(int32(udrContext.SBIPort))
	services := []models.NFService{
		{
			ServiceInstanceId: "datarepository",
			ServiceName:       models.SERVICENAME_NUDR_DR,
			Versions: []models.NFServiceVersion{
				{
					ApiFullVersion:  version,
					ApiVersionInUri: versionUri,
				},
			},
			Scheme:          udrContext.UriScheme,
			NfServiceStatus: models.NFSERVICESTATUS_REGISTERED,
			ApiPrefix:       openapi.PtrString(apiPrefix),
			IpEndPoints:     []models.IpEndPoint{*ipEndPoint},
		},
	}
	profile.NfServices = services
	// TODO: finish the Udr Info
	profile.UdrInfo = &models.UdrInfo{
		SupportedDataSets: []models.DataSetId{
			// models.DataSetId_APPLICATION,
			// models.DataSetId_EXPOSURE,
			// models.DataSetId_POLICY,
			models.DATASETID_SUBSCRIPTION,
		},
	}
	return &profile, nil
}

var SendRegisterNFInstance = func(plmnConfig []models.PlmnId) (prof *models.NFProfile, resourceNrfUri string, err error) {
	udrSelf := udrContext.UDR_Self()
	nfProfile, err := getNfProfile(udrSelf, plmnConfig)
	if err != nil {
		return &models.NFProfile{}, "", err
	}
	configuration := Nnrf_NFManagement.NewConfiguration()
	serverConfig := &configuration.Servers[0]
	if apiRootVar, exists := serverConfig.Variables["apiRoot"]; exists {
		apiRootVar.DefaultValue = udrSelf.NrfUri
		serverConfig.Variables["apiRoot"] = apiRootVar
	}
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	apiRegisterNFInstanceRequest := client.NFInstanceIDDocumentAPI.RegisterNFInstance(context.TODO(), nfProfile.NfInstanceId)
	apiRegisterNFInstanceRequest = apiRegisterNFInstanceRequest.NFProfile(*nfProfile)
	receivedNfProfile, res, err := client.NFInstanceIDDocumentAPI.RegisterNFInstanceExecute(apiRegisterNFInstanceRequest)
	if err != nil {
		return &models.NFProfile{}, "", err
	}
	if res == nil {
		return &models.NFProfile{}, "", fmt.Errorf("no response from server")
	}

	switch res.StatusCode {
	case http.StatusOK: // NFUpdate
		logger.ConsumerLog.Debugln("UDR NF profile updated with complete replacement")
		return receivedNfProfile, "", nil
	case http.StatusCreated: // NFRegister
		resourceUri := res.Header.Get("Location")
		resourceNrfUri = resourceUri[:strings.Index(resourceUri, "/nnrf-nfm/")]
		retrieveNfInstanceId := resourceUri[strings.LastIndex(resourceUri, "/")+1:]
		udrSelf.NfId = retrieveNfInstanceId
		logger.ConsumerLog.Debugln("UDR NF profile registered to the NRF")
		return receivedNfProfile, resourceNrfUri, nil
	default:
		return receivedNfProfile, "", fmt.Errorf("unexpected status code returned by the NRF %d", res.StatusCode)
	}
}

var SendDeregisterNFInstance = func() error {
	logger.ConsumerLog.Infoln("send Deregister NFInstance")

	udrSelf := udrContext.UDR_Self()
	// Set client and set url
	configuration := Nnrf_NFManagement.NewConfiguration()
	serverConfig := &configuration.Servers[0]
	if apiRootVar, exists := serverConfig.Variables["apiRoot"]; exists {
		apiRootVar.DefaultValue = udrSelf.NrfUri
		serverConfig.Variables["apiRoot"] = apiRootVar
	}
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	apiDeregisterNFInstanceRequest := client.NFInstanceIDDocumentAPI.DeregisterNFInstance(context.Background(), udrSelf.NfId)
	res, err := client.NFInstanceIDDocumentAPI.DeregisterNFInstanceExecute(apiDeregisterNFInstanceRequest)
	if err != nil {
		return err
	}
	if res == nil {
		return fmt.Errorf("no response from server")
	}
	if res.StatusCode == http.StatusNoContent {
		return nil
	}
	return fmt.Errorf("unexpected response code")
}

var SendUpdateNFInstance = func(patchItem []models.PatchItem) (receivedNfProfile *models.NFProfile, problemDetails *models.ProblemDetails, err error) {
	logger.ConsumerLog.Debugln("send Update NFInstance")

	udrSelf := udrContext.UDR_Self()
	configuration := Nnrf_NFManagement.NewConfiguration()
	serverConfig := &configuration.Servers[0]
	if apiRootVar, exists := serverConfig.Variables["apiRoot"]; exists {
		apiRootVar.DefaultValue = udrSelf.NrfUri
		serverConfig.Variables["apiRoot"] = apiRootVar
	}
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	var res *http.Response
	apiUpdateNFInstanceRequest := client.NFInstanceIDDocumentAPI.UpdateNFInstance(context.Background(), udrSelf.NfId)
	apiUpdateNFInstanceRequest = apiUpdateNFInstanceRequest.PatchItem(patchItem)
	receivedNfProfile, res, err = client.NFInstanceIDDocumentAPI.UpdateNFInstanceExecute(apiUpdateNFInstanceRequest)
	if err != nil {
		if openapiErr, ok := err.(openapi.GenericOpenAPIError); ok {
			if model := openapiErr.Model(); model != nil {
				if problem, ok := model.(models.ProblemDetails); ok {
					return &models.NFProfile{}, &problem, nil
				}
			}
		}
		return &models.NFProfile{}, nil, err
	}

	if res == nil {
		return &models.NFProfile{}, nil, fmt.Errorf("no response from server")
	}
	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusNoContent {
		return receivedNfProfile, nil, nil
	}
	return &models.NFProfile{}, nil, fmt.Errorf("unexpected response code")
}
