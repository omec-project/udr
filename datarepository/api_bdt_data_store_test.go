// Copyright (c) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package datarepository

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/udr/producer"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type emptyBdtDB struct{ producer.DBInterface }

func (emptyBdtDB) RestfulAPIGetMany(_ string, _ bson.M) ([]map[string]any, error) {
	return nil, nil
}

// PCF BDT policy creation reads this collection; it must not be a 501 stub.
func TestHTTPReadBdtData_ReturnsEmptyListNotNotImplemented(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prev := producer.CommonDBClient
	producer.CommonDBClient = emptyBdtDB{}
	t.Cleanup(func() { producer.CommonDBClient = prev })

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/policy-data/bdt-data", nil)

	HTTPReadBdtData(ginCtx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d with body %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != "[]" {
		t.Fatalf("expected body [], got %q", body)
	}
}
