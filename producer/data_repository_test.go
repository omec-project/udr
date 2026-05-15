// Copyright (c) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"testing"

	"github.com/omec-project/openapi/v2/models"
	"go.mongodb.org/mongo-driver/bson"
)

func TestSnssaiEqualUsesSdValueSemantics(t *testing.T) {
	left := models.Snssai{Sst: 1}
	left.SetSd("ABCDEF")
	right := models.Snssai{Sst: 1}
	right.SetSd("ABCDEF")

	if !snssaiEqual(left, right) {
		t.Fatal("expected snssaiEqual to match identical S-NSSAI values even with different Sd pointers")
	}
}

func TestAddSingleNssaiFilterStoresSdAsString(t *testing.T) {
	filter := bson.M{}
	singleNssai := models.Snssai{Sst: 1}
	singleNssai.SetSd("ABCDEF")

	addSingleNssaiFilter(filter, singleNssai)

	if got, ok := filter["singlenssai.sd"].(string); !ok || got != "ABCDEF" {
		t.Fatalf("expected singlenssai.sd filter to be stored as string value, got %#v", filter["singlenssai.sd"])
	}
	if got, ok := filter["singlenssai.sst"].(int32); !ok || got != 1 {
		t.Fatalf("expected singlenssai.sst filter to be stored as int32 value, got %#v", filter["singlenssai.sst"])
	}
}
