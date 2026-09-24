// Copyright (c) 2026 Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
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

func closeNFManagementResponseBody(res *http.Response, operation string) {
	if res == nil || res.Body == nil {
		return
	}
	if bodyCloseErr := res.Body.Close(); bodyCloseErr != nil {
		logger.ConsumerLog.Errorf("%s response body cannot close: %+v", operation, bodyCloseErr)
	}
}

func getNfProfile(udrContext *udrContext.UDRContext, plmnConfig []models.PlmnId) (profile models.NFProfile, err error) {
	if udrContext == nil {
		return profile, openapi.ReportError("udr context has not been initialized. NF profile cannot be built")
	}
	profile = *models.NewNFProfileWithDefaults()
	profile.SetNfInstanceId(udrContext.NfId)
	profile.SetNfType(models.NFTYPE_UDR)
	profile.SetNfStatus(models.NFSTATUS_REGISTERED)
	if len(plmnConfig) > 0 {
		plmnCopy := make([]models.PlmnId, len(plmnConfig))
		copy(plmnCopy, plmnConfig)
		profile.SetPlmnList(plmnCopy)
	}

	version := factory.UdrConfig.Info.Version
	tmpVersion := strings.Split(version, ".")
	versionUri := "v" + tmpVersion[0]
	apiPrefix := fmt.Sprintf("%s://%s:%d", udrContext.UriScheme, udrContext.RegisterIPv4, udrContext.SBIPort)
	ipEndPoint := models.NewIpEndPoint()
	ipEndPoint.SetIpv4Address(udrContext.RegisterIPv4)
	ipEndPoint.SetTransport(models.TRANSPORTPROTOCOL_TCP)
	ipEndPoint.SetPort(int32(udrContext.SBIPort))
	nfServiceVersion := models.NewNFServiceVersion(versionUri, version)
	nfService := models.NewNFService("datarepository", models.SERVICENAME_NUDR_DR, []models.NFServiceVersion{*nfServiceVersion}, udrContext.UriScheme, models.NFSERVICESTATUS_REGISTERED)
	nfService.SetApiPrefix(apiPrefix)
	nfService.SetIpEndPoints([]models.IpEndPoint{*ipEndPoint})
	profile.SetNfServices([]models.NFService{*nfService})
	profile.SetNfServiceList(map[string]models.NFService{
		nfService.GetServiceInstanceId(): *nfService,
	})
	udrInfo := models.NewUdrInfo()
	udrInfo.SetSupportedDataSets([]models.DataSetId{
		models.DATASETID_SUBSCRIPTION,
	})
	profile.SetUdrInfo(*udrInfo)
	return profile, nil
}

var SendRegisterNFInstance = func(plmnConfig []models.PlmnId) (prof *models.NFProfile, resourceNrfUri string, err error) {
	self := udrContext.UDR_Self()
	nfProfile, err := getNfProfile(self, plmnConfig)
	if err != nil {
		return models.NewNFProfileWithDefaults(), "", err
	}

	configuration := Nnrf_NFManagement.NewConfiguration()
	serverConfig := &configuration.Servers[0]
	if apiRootVar, exists := serverConfig.Variables["apiRoot"]; exists {
		apiRootVar.DefaultValue = self.NrfUri
		serverConfig.Variables["apiRoot"] = apiRootVar
	}
	client := Nnrf_NFManagement.NewAPIClient(configuration)
	apiRegisterNFInstanceRequest := client.NFInstanceIDDocumentAPI.RegisterNFInstance(context.TODO(), nfProfile.GetNfInstanceId())
	apiRegisterNFInstanceRequest = apiRegisterNFInstanceRequest.NFProfile(nfProfile)
	receivedNfProfile, res, err := client.NFInstanceIDDocumentAPI.RegisterNFInstanceExecute(apiRegisterNFInstanceRequest)
	defer closeNFManagementResponseBody(res, "RegisterNFInstance")
	logger.ConsumerLog.Debugf("registering NF Instance using profile: %+v", nfProfile)

	if err != nil {
		return models.NewNFProfileWithDefaults(), "", err
	}
	if res == nil {
		return models.NewNFProfileWithDefaults(), "", openapi.ReportError("no response from server")
	}

	switch res.StatusCode {
	case http.StatusOK:
		logger.ConsumerLog.Debugln("UDR NF profile updated with complete replacement")
		return receivedNfProfile, "", nil
	case http.StatusCreated:
		resourceUri := res.Header.Get("Location")
		resourceNrfUri = resourceUri[:strings.Index(resourceUri, "/nnrf-nfm/")]
		retrieveNfInstanceId := resourceUri[strings.LastIndex(resourceUri, "/")+1:]
		self.NfId = retrieveNfInstanceId
		logger.ConsumerLog.Debugln("UDR NF profile registered to the NRF")
		return receivedNfProfile, resourceNrfUri, nil
	default:
		return receivedNfProfile, "", openapi.ReportError("NRF returned unexpected status code %d", res.StatusCode)
	}
}

var SendDeregisterNFInstance = func() error {
	logger.ConsumerLog.Infoln("send Deregister NFInstance")

	self := udrContext.UDR_Self()
	// Set client and set url
	configuration := Nnrf_NFManagement.NewConfiguration()
	serverConfig := &configuration.Servers[0]
	if apiRootVar, exists := serverConfig.Variables["apiRoot"]; exists {
		apiRootVar.DefaultValue = self.NrfUri
		serverConfig.Variables["apiRoot"] = apiRootVar
	}
	client := Nnrf_NFManagement.NewAPIClient(configuration)
	apiDeregisterNFInstanceRequest := client.NFInstanceIDDocumentAPI.DeregisterNFInstance(context.Background(), self.NfId)
	res, err := client.NFInstanceIDDocumentAPI.DeregisterNFInstanceExecute(apiDeregisterNFInstanceRequest)
	defer closeNFManagementResponseBody(res, "DeregisterNFInstance")
	if err != nil {
		return err
	}
	if res == nil {
		return openapi.ReportError("no response from server")
	}
	if res.StatusCode == http.StatusNoContent {
		return nil
	}
	return openapi.ReportError("unexpected response code")
}

var SendUpdateNFInstance = func(patchItem []models.PatchItem) (receivedNfProfile *models.NFProfile, problemDetails *models.ProblemDetails, err error) {
	logger.ConsumerLog.Debugln("send Update NFInstance")

	self := udrContext.UDR_Self()
	configuration := Nnrf_NFManagement.NewConfiguration()
	serverConfig := &configuration.Servers[0]
	if apiRootVar, exists := serverConfig.Variables["apiRoot"]; exists {
		apiRootVar.DefaultValue = self.NrfUri
		serverConfig.Variables["apiRoot"] = apiRootVar
	}
	client := Nnrf_NFManagement.NewAPIClient(configuration)

	var res *http.Response
	apiUpdateNFInstanceRequest := client.NFInstanceIDDocumentAPI.UpdateNFInstance(context.Background(), self.NfId)
	apiUpdateNFInstanceRequest = apiUpdateNFInstanceRequest.PatchItem(patchItem)
	receivedNfProfile, res, err = client.NFInstanceIDDocumentAPI.UpdateNFInstanceExecute(apiUpdateNFInstanceRequest)
	defer closeNFManagementResponseBody(res, "UpdateNFInstance")
	if err != nil {
		if openapiErr, ok := openapi.AsGenericOpenAPIError(err); ok {
			if model := openapiErr.Model(); model != nil {
				if problem, ok := model.(models.ProblemDetails); ok {
					return models.NewNFProfileWithDefaults(), &problem, nil
				}
			}
		}
		return models.NewNFProfileWithDefaults(), nil, err
	}

	if res == nil {
		return models.NewNFProfileWithDefaults(), nil, openapi.ReportError("no response from server")
	}
	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusNoContent {
		return receivedNfProfile, nil, nil
	}
	return models.NewNFProfileWithDefaults(), nil, openapi.ReportError("unexpected response code %d", res.StatusCode)
}
