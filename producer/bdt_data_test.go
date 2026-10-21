// Copyright (c) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"errors"
	"net/http"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// bdtStubDB overrides RestfulAPIGetMany on top of stubDB.
type bdtStubDB struct {
	stubDB
	many []map[string]any
	err  error
}

func (s *bdtStubDB) RestfulAPIGetMany(_ string, _ bson.M) ([]map[string]any, error) {
	return s.many, s.err
}

func withCommonDB(t *testing.T, db DBInterface) {
	t.Helper()
	prev := CommonDBClient
	CommonDBClient = db
	t.Cleanup(func() { CommonDBClient = prev })
}

func TestPolicyDataBdtDataGetProcedure_EmptyCollectionIsEmptyList(t *testing.T) {
	withCommonDB(t, &bdtStubDB{})

	response, problemDetails := PolicyDataBdtDataGetProcedure(POLICYDATA_BDTDATA)
	if problemDetails != nil {
		t.Fatalf("unexpected problem details: %+v", problemDetails)
	}
	if response == nil || *response == nil {
		t.Fatal("expected a non-nil empty list so the body serializes as [] rather than null")
	}
	if len(*response) != 0 {
		t.Fatalf("expected empty list, got %d entries", len(*response))
	}
}

func TestPolicyDataBdtDataGetProcedure_ReturnsStoredEntries(t *testing.T) {
	withCommonDB(t, &bdtStubDB{many: []map[string]any{{"bdtRefId": "ref-1"}}})

	response, problemDetails := PolicyDataBdtDataGetProcedure(POLICYDATA_BDTDATA)
	if problemDetails != nil {
		t.Fatalf("unexpected problem details: %+v", problemDetails)
	}
	if len(*response) != 1 || (*response)[0]["bdtRefId"] != "ref-1" {
		t.Fatalf("unexpected response: %+v", *response)
	}
}

func TestHandlePolicyDataBdtDataGet_DBErrorIsNotReportedAsSuccess(t *testing.T) {
	withCommonDB(t, &bdtStubDB{err: errors.New("db down")})

	rsp := HandlePolicyDataBdtDataGet(nil)
	if rsp.Status != http.StatusInternalServerError {
		t.Fatalf("expected %d on DB error, got %d", http.StatusInternalServerError, rsp.Status)
	}
}
