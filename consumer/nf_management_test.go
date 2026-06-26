// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Open Networking Foundation <info@opennetworking.org>
package consumer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/omec-project/openapi/v2/models"
	udrContext "github.com/omec-project/udr/context"
	"github.com/omec-project/udr/factory"
)

func Test_getNfProfile_sets_subscription_udr_info(t *testing.T) {
	origConfig := factory.UdrConfig
	t.Cleanup(func() { factory.UdrConfig = origConfig })
	factory.UdrConfig = factory.Config{
		Info: &factory.Info{Version: "1.0.0"},
	}

	prof, err := getNfProfile(&udrContext.UDRContext{
		NfId:         "test-udr",
		UriScheme:    models.URISCHEME_HTTP,
		RegisterIPv4: "127.0.0.10",
		SBIPort:      8000,
	}, []models.PlmnId{{Mcc: "123", Mnc: "45"}})
	if err != nil {
		t.Fatalf("getNfProfile returned error: %v", err)
	}

	udrInfo, ok := prof.GetUdrInfoOk()
	if !ok {
		t.Fatal("expected UdrInfo to be set on NF profile")
	}

	supportedDataSets := udrInfo.GetSupportedDataSets()
	if len(supportedDataSets) != 1 {
		t.Fatalf("expected one supported data set, got %d", len(supportedDataSets))
	}
	if supportedDataSets[0] != models.DATASETID_SUBSCRIPTION {
		t.Fatalf("expected supported data set %q, got %q", models.DATASETID_SUBSCRIPTION, supportedDataSets[0])
	}
}

func Test_nrf_url_is_not_overwritten_when_registering(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/nnrf-nfm/v1/nf-instances/") {
			w.Header().Set("Location", fmt.Sprintf("%s/nnrf-nfm/v1/nf-instances/mocked-id", r.Host))
			w.WriteHeader(http.StatusCreated)
		} else {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	}))
	defer svr.Close()
	if err := factory.InitConfigFactory("../factory/udr_config.yaml"); err != nil {
		t.Fatalf("Could not read example configuration file")
	}
	self := udrContext.UDR_Self()
	self.NrfUri = svr.URL
	self.RegisterIPv4 = "127.0.0.2"

	_, _, err := SendRegisterNFInstance([]models.PlmnId{{Mcc: "123", Mnc: "45"}})
	if err != nil {
		t.Errorf("Got and error %+v", err)
	}
	if self.NfId != "mocked-id" {
		t.Errorf("Expected NfId to be 'mocked-id', got %v", self.NfId)
	}
	if self.NrfUri != svr.URL {
		t.Errorf("Expected NRF URL to stay %s, but was %s", svr.URL, self.NrfUri)
	}
}
