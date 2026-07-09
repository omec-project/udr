// Copyright (c) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"testing"

	"github.com/omec-project/openapi/v2/models"
	"go.mongodb.org/mongo-driver/bson"
)

const testHexSnssai = "01ABCDEF"

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

func TestAddSmPolicySnssaiDnnFilterWithDottedDnn(t *testing.T) {
	filter := bson.M{}
	hexSnssai := testHexSnssai
	dnn := "internet.example"

	addSmPolicySnssaiDnnFilter(filter, hexSnssai, dnn)

	exprVal, ok := filter["$expr"]
	if !ok {
		t.Fatal("expected $expr key in filter when dnn is non-empty")
	}
	exprMap, ok := exprVal.(bson.M)
	if !ok {
		t.Fatalf("expected $expr to be bson.M, got %T", exprVal)
	}
	inVal, ok := exprMap["$in"]
	if !ok {
		t.Fatal("expected $in key inside $expr for dotted DNN")
	}
	inArr, ok := inVal.(bson.A)
	if !ok || len(inArr) != 2 {
		t.Fatalf("expected $in to be bson.A with 2 elements, got %#v", inVal)
	}
	literalMap, ok := inArr[0].(bson.M)
	if !ok {
		t.Fatalf("expected first $in element to be bson.M, got %T", inArr[0])
	}
	if literalMap["$literal"] != dnn {
		t.Fatalf("expected $literal to be %q, got %#v", dnn, literalMap["$literal"])
	}
}

func TestAddSmPolicySnssaiDnnFilterWithoutDnn(t *testing.T) {
	filter := bson.M{}
	hexSnssai := testHexSnssai

	addSmPolicySnssaiDnnFilter(filter, hexSnssai, "")

	expectedKey := "smPolicySnssaiData." + hexSnssai
	val, ok := filter[expectedKey]
	if !ok {
		t.Fatalf("expected key %q in filter when dnn is empty", expectedKey)
	}
	existsMap, ok := val.(bson.M)
	if !ok || existsMap["$exists"] != true {
		t.Fatalf("expected $exists:true filter, got %#v", val)
	}
}

func TestAddDotSafeKeyExistsFilterStructure(t *testing.T) {
	filter := bson.M{}
	addDotSafeKeyExistsFilter(filter, "dnnconfigurations", "internet.example")

	exprMap, ok := filter["$expr"].(bson.M)
	if !ok {
		t.Fatal("expected $expr to be bson.M")
	}
	inArr, ok := exprMap["$in"].(bson.A)
	if !ok || len(inArr) != 2 {
		t.Fatalf("expected $in bson.A with 2 elements, got %#v", exprMap["$in"])
	}
	literalMap, ok := inArr[0].(bson.M)
	if !ok || literalMap["$literal"] != "internet.example" {
		t.Fatalf("expected $literal to be %q, got %#v", "internet.example", inArr[0])
	}
	mapExpr, ok := inArr[1].(bson.M)
	if !ok {
		t.Fatalf("expected second $in element to be bson.M, got %T", inArr[1])
	}
	if _, ok := mapExpr["$map"]; !ok {
		t.Fatal("expected $map operator in second $in element")
	}
}

func TestAddSmPolicySnssaiDnnFilterWithDotFreeDnn(t *testing.T) {
	filter := bson.M{}
	hexSnssai := testHexSnssai
	dnn := "internet"

	addSmPolicySnssaiDnnFilter(filter, hexSnssai, dnn)

	expectedKey := "smPolicySnssaiData." + hexSnssai + ".smPolicyDnnData." + dnn
	val, ok := filter[expectedKey]
	if !ok {
		t.Fatalf("expected key %q in filter for dot-free dnn", expectedKey)
	}
	existsMap, ok := val.(bson.M)
	if !ok || existsMap["$exists"] != true {
		t.Fatalf("expected $exists:true filter for dot-free dnn, got %#v", val)
	}
	if _, hasExpr := filter["$expr"]; hasExpr {
		t.Fatal("expected no $expr filter for dot-free dnn")
	}
}

func TestAddDotSafeKeyExistsFilterMergesExistingExpr(t *testing.T) {
	existingExpr := bson.M{"$eq": bson.A{"$status", "active"}}
	filter := bson.M{"$expr": existingExpr}

	addDotSafeKeyExistsFilter(filter, "dnnconfigurations", "internet.example")

	andExpr, ok := filter["$expr"].(bson.M)
	if !ok {
		t.Fatalf("expected $expr to be bson.M after merge, got %T", filter["$expr"])
	}
	andArr, ok := andExpr["$and"].(bson.A)
	if !ok || len(andArr) != 2 {
		t.Fatalf("expected $and with 2 elements, got %#v", andExpr["$and"])
	}
	first, ok := andArr[0].(bson.M)
	if !ok {
		t.Fatalf("expected first $and element to be bson.M, got %T", andArr[0])
	}
	if _, hasEq := first["$eq"]; !hasEq {
		t.Fatal("expected original $expr ($eq) to be preserved as first $and element")
	}
	newExpr, ok := andArr[1].(bson.M)
	if !ok {
		t.Fatalf("expected new $in expr as second $and element, got %T", andArr[1])
	}
	if _, hasIn := newExpr["$in"]; !hasIn {
		t.Fatal("expected $in operator in merged $expr")
	}
}
