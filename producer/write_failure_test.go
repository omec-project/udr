// Copyright (C) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/omec-project/openapi/v2/models"
	udr_context "github.com/omec-project/udr/context"
	"github.com/omec-project/util/httpwrapper"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// writeStubDB overrides the write and delete methods of stubDB so a test can
// make them fail. Unless existed is set, no document is reported as already
// stored, so a successful PUT answers 201/204 and no change notification is sent.
type writeStubDB struct {
	stubDB
	writeErr error
	existed  bool
}

func (s *writeStubDB) RestfulAPIPutOne(_ string, _ bson.M, _ map[string]any) (bool, error) {
	return s.existed, s.writeErr
}

func (s *writeStubDB) RestfulAPIDeleteOne(_ string, _ bson.M) error { return s.writeErr }

const (
	testUeId         = "imsi-001010000000001"
	testPduSessionId = "1"
)

func ueRequest(body any) *httpwrapper.Request {
	return &httpwrapper.Request{
		Params: map[string]string{
			ParamUeId:           testUeId,
			ParamPduSessionId:   testPduSessionId,
			ParamBdtReferenceId: "bdt-1",
			ParamUsageMonId:     "mon-1",
		},
		Body: body,
	}
}

// writeHandlerCases lists every handler whose database write or delete result
// must reach the response. okStatus is the status expected when the write succeeds.
var writeHandlerCases = []struct {
	name     string
	call     func() *httpwrapper.Response
	okStatus int
}{
	{"CreateAmfContext3gpp", func() *httpwrapper.Response {
		return HandleCreateAmfContext3gpp(ueRequest(models.Amf3GppAccessRegistration{}))
	}, http.StatusNoContent},
	{"CreateAmfContextNon3gpp", func() *httpwrapper.Response {
		return HandleCreateAmfContextNon3gpp(ueRequest(models.AmfNon3GppAccessRegistration{}))
	}, http.StatusNoContent},
	{"CreateAuthenticationSoR", func() *httpwrapper.Response {
		return HandleCreateAuthenticationSoR(ueRequest(map[string]any{}))
	}, http.StatusNoContent},
	{"CreateAuthenticationStatus", func() *httpwrapper.Response {
		return HandleCreateAuthenticationStatus(ueRequest(map[string]any{}))
	}, http.StatusNoContent},
	{"InfluenceDataDelete", func() *httpwrapper.Response {
		return HandleApplicationDataInfluenceDataInfluenceIdDelete("infl-1")
	}, http.StatusNoContent},
	{"InfluenceDataPatch", func() *httpwrapper.Response {
		return HandleApplicationDataInfluenceDataInfluenceIdPatch("infl-1", &models.TrafficInfluDataPatch{})
	}, http.StatusOK},
	{"InfluenceDataPut", func() *httpwrapper.Response {
		return HandleApplicationDataInfluenceDataInfluenceIdPut("infl-1", &models.TrafficInfluData{})
	}, http.StatusCreated},
	{"InfluenceDataSubsPut", func() *httpwrapper.Response {
		return HandleApplicationDataInfluenceDataSubsToNotifySubscriptionIdPut("sub-1", &models.TrafficInfluSub{})
	}, http.StatusOK},
	{"InfluenceDataSubsDelete", func() *httpwrapper.Response {
		return HandleApplicationDataInfluenceDataSubsToNotifySubscriptionIdDelete("sub-1")
	}, http.StatusNoContent},
	{"PfdsDelete", func() *httpwrapper.Response {
		return HandleApplicationDataPfdsAppIdDelete("app-1")
	}, http.StatusNoContent},
	{"PfdsPut", func() *httpwrapper.Response {
		return HandleApplicationDataPfdsAppIdPut("app-1", &models.PfdDataForApp{})
	}, http.StatusCreated},
	{"BdtDataDelete", func() *httpwrapper.Response {
		return HandlePolicyDataBdtDataBdtReferenceIdDelete(ueRequest(nil))
	}, http.StatusNoContent},
	{"BdtDataPut", func() *httpwrapper.Response {
		return HandlePolicyDataBdtDataBdtReferenceIdPut(ueRequest(models.BdtData{}))
	}, http.StatusOK},
	{"OperatorSpecificDataPut", func() *httpwrapper.Response {
		return HandlePolicyDataUesUeIdOperatorSpecificDataPut(
			ueRequest(map[string]models.OperatorSpecificDataContainer{}))
	}, http.StatusOK},
	{"UsageMonDelete", func() *httpwrapper.Response {
		return HandlePolicyDataUesUeIdSmDataUsageMonIdDelete(ueRequest(nil))
	}, http.StatusNoContent},
	{"UsageMonPut", func() *httpwrapper.Response {
		return HandlePolicyDataUesUeIdSmDataUsageMonIdPut(ueRequest(models.UsageMonData{}))
	}, http.StatusCreated},
	{"UePolicySetPut", func() *httpwrapper.Response {
		return HandlePolicyDataUesUeIdUePolicySetPut(ueRequest(models.UePolicySet{}))
	}, http.StatusCreated},
	{"CreateSmfContextNon3gpp", func() *httpwrapper.Response {
		return HandleCreateSmfContextNon3gpp(ueRequest(models.SmfRegistration{}))
	}, http.StatusCreated},
	{"DeleteSmfContext", func() *httpwrapper.Response {
		return HandleDeleteSmfContext(ueRequest(nil))
	}, http.StatusNoContent},
	{"CreateSmsfContext3gpp", func() *httpwrapper.Response {
		return HandleCreateSmsfContext3gpp(ueRequest(models.SmsfRegistration{}))
	}, http.StatusNoContent},
	{"DeleteSmsfContext3gpp", func() *httpwrapper.Response {
		return HandleDeleteSmsfContext3gpp(ueRequest(nil))
	}, http.StatusNoContent},
	{"CreateSmsfContextNon3gpp", func() *httpwrapper.Response {
		return HandleCreateSmsfContextNon3gpp(ueRequest(models.SmsfRegistration{}))
	}, http.StatusNoContent},
	{"DeleteSmsfContextNon3gpp", func() *httpwrapper.Response {
		return HandleDeleteSmsfContextNon3gpp(ueRequest(nil))
	}, http.StatusNoContent},
}

func TestWriteHandlers_DBFailureIsReportedAsSystemFailure(t *testing.T) {
	for _, tc := range writeHandlerCases {
		t.Run(tc.name, func(t *testing.T) {
			db := &writeStubDB{writeErr: errors.New("db down")}
			db.result = map[string]any{"afAppId": "af-1"}
			withCommonDB(t, db)

			rsp := tc.call()
			if rsp.Status != http.StatusInternalServerError {
				t.Fatalf("expected %d on DB write failure, got %d", http.StatusInternalServerError, rsp.Status)
			}
			pd, ok := rsp.Body.(*models.ProblemDetails)
			if !ok {
				t.Fatalf("expected *models.ProblemDetails body, got %T", rsp.Body)
			}
			if pd.GetCause() != "SYSTEM_FAILURE" {
				t.Fatalf("expected cause SYSTEM_FAILURE, got %q", pd.GetCause())
			}
		})
	}
}

func TestWriteHandlers_SuccessKeepsExistingStatus(t *testing.T) {
	for _, tc := range writeHandlerCases {
		t.Run(tc.name, func(t *testing.T) {
			db := &writeStubDB{}
			db.result = map[string]any{"afAppId": "af-1"}
			withCommonDB(t, db)

			if rsp := tc.call(); rsp.Status != tc.okStatus {
				t.Fatalf("expected %d on DB write success, got %d", tc.okStatus, rsp.Status)
			}
		})
	}
}

func TestInfluenceDataSubsToNotifyPost_DBFailureHasNoLocation(t *testing.T) {
	db := &writeStubDB{writeErr: errors.New("db down")}
	withCommonDB(t, db)

	rsp := HandleApplicationDataInfluenceDataSubsToNotifyPost(&models.TrafficInfluSub{})
	if rsp.Status != http.StatusInternalServerError {
		t.Fatalf("expected %d on DB write failure, got %d", http.StatusInternalServerError, rsp.Status)
	}
	pd, ok := rsp.Body.(*models.ProblemDetails)
	if !ok || pd.GetCause() != "SYSTEM_FAILURE" {
		t.Fatalf("expected SYSTEM_FAILURE problem details, got %#v", rsp.Body)
	}
	if loc := rsp.Header.Get("Location"); loc != "" {
		t.Fatalf("expected no Location header on failure, got %q", loc)
	}
}

func TestInfluenceDataSubsToNotifyPost_SuccessKeepsCreatedAndLocation(t *testing.T) {
	withCommonDB(t, &writeStubDB{})

	rsp := HandleApplicationDataInfluenceDataSubsToNotifyPost(&models.TrafficInfluSub{})
	if rsp.Status != http.StatusCreated {
		t.Fatalf("expected %d on DB write success, got %d", http.StatusCreated, rsp.Status)
	}
	if loc := rsp.Header.Get("Location"); !strings.Contains(loc, "/application-data/influenceData/subs-to-notify/") {
		t.Fatalf("expected subs-to-notify Location header, got %q", loc)
	}
}

// withPolicyDataSubscriber registers a policy-data subscriber backed by an
// httptest server and returns a channel that receives one value per notification.
func withPolicyDataSubscriber(t *testing.T) <-chan struct{} {
	t.Helper()
	hits := make(chan struct{}, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	udrSelf := udr_context.UDR_Self()
	original := udrSelf.PolicyDataSubscriptions
	udrSelf.PolicyDataSubscriptions = map[string]*models.PolicyDataSubscription{
		"1": models.NewPolicyDataSubscription(server.URL, []string{"/nudr-dr/v2/policy-data"}),
	}
	t.Cleanup(func() { udrSelf.PolicyDataSubscriptions = original })
	return hits
}

func TestPolicyDataBdtDataPut_ExistingRecordNotifiesOnlyOnSuccess(t *testing.T) {
	t.Run("successful update notifies", func(t *testing.T) {
		hits := withPolicyDataSubscriber(t)
		withCommonDB(t, &writeStubDB{existed: true})

		response, err := PolicyDataBdtDataBdtReferenceIdPutProcedure(POLICYDATA_BDTDATA, "bdt-1", models.BdtData{})
		if err != nil || response == nil {
			t.Fatalf("expected a response and no error, got (%v, %v)", response, err)
		}
		select {
		case <-hits:
		case <-time.After(2 * time.Second):
			t.Fatal("expected a notification after a successful update of an existing record")
		}
	})

	t.Run("failed update does not notify", func(t *testing.T) {
		hits := withPolicyDataSubscriber(t)
		withCommonDB(t, &writeStubDB{existed: true, writeErr: errors.New("db down")})

		response, err := PolicyDataBdtDataBdtReferenceIdPutProcedure(POLICYDATA_BDTDATA, "bdt-1", models.BdtData{})
		if err == nil || response != nil {
			t.Fatalf("expected (nil, error) on DB failure, got (%v, %v)", response, err)
		}
		select {
		case <-hits:
			t.Fatal("unexpected notification after a failed update")
		case <-time.After(300 * time.Millisecond):
		}
	})
}
