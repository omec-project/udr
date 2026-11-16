// Copyright (C) 2026 Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"fmt"
	"strings"

	"github.com/omec-project/openapi/v2"
	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/openapi/v2/nrfclient"
	udrContext "github.com/omec-project/udr/context"
	"github.com/omec-project/udr/factory"
	"github.com/omec-project/udr/logger"
)

func newNrfClient(nrfUri string) *nrfclient.Client {
	return nrfclient.New(nrfUri, nrfclient.WithLogger(logger.ConsumerLog))
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

// SendRegisterNFInstance and SendDeregisterNFInstance take the context of the registration service, which holds
// its registration mutex while they run and cancels the context to end them when a new configuration arrives or
// the NF terminates.
var SendRegisterNFInstance = func(ctx context.Context, plmnConfig []models.PlmnId) (prof *models.NFProfile, resourceNrfUri string, err error) {
	self := udrContext.UDR_Self()
	nfProfile, err := getNfProfile(self, plmnConfig)
	if err != nil {
		return models.NewNFProfileWithDefaults(), "", err
	}

	result, err := newNrfClient(self.NrfUri).Register(ctx, nfProfile)
	if err != nil {
		return models.NewNFProfileWithDefaults(), "", err
	}
	if result.Created {
		logger.ConsumerLog.Debugln("UDR NF profile registered to the NRF")
		if result.NfInstanceID != "" {
			self.NfId = result.NfInstanceID
		}
	} else {
		logger.ConsumerLog.Debugln("UDR NF profile updated with complete replacement")
	}
	return result.Profile, result.NrfURI, nil
}

var SendDeregisterNFInstance = func(ctx context.Context) error {
	logger.ConsumerLog.Infoln("send Deregister NFInstance")

	self := udrContext.UDR_Self()
	return newNrfClient(self.NrfUri).Deregister(ctx, self.NfId)
}

var SendUpdateNFInstance = func(ctx context.Context, patchItem []models.PatchItem) (receivedNfProfile *models.NFProfile, problemDetails *models.ProblemDetails, err error) {
	logger.ConsumerLog.Debugln("send Update NFInstance")

	self := udrContext.UDR_Self()
	return newNrfClient(self.NrfUri).Update(ctx, self.NfId, patchItem)
}
