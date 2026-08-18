// Copyright (c) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"fmt"
	"sync"
	"testing"

	"github.com/omec-project/openapi/v2/models"
	udr_context "github.com/omec-project/udr/context"
	"go.mongodb.org/mongo-driver/v2/bson"
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

// TestCreateSdmSubscriptionsProcedureIsConcurrencySafe reproduces the crash
// seen on a live core once registration concurrency rose: the UDM creates an
// SDM subscription per registration, and unsynchronised access to
// UESubsData.SdmSubscriptions aborted the process with "concurrent map
// writes". Run with -race to also catch the shared ID generator.
func TestCreateSdmSubscriptionsProcedureIsConcurrencySafe(t *testing.T) {
	const (
		ueCount       = 8
		perUeRequests = 64
	)

	// UESubsCollection is a process-wide singleton, so leaving these entries
	// behind would make a second run in the same process (go test -count=2, or
	// any later test reusing these IDs) see the accumulated subscriptions and
	// fail the count assertion.
	udrSelf := udr_context.UDR_Self()
	ueIds := make([]string, 0, ueCount)
	for u := range ueCount {
		ueIds = append(ueIds, fmt.Sprintf("imsi-20893010000%04d", u))
	}
	clearUeSubs := func() {
		for _, id := range ueIds {
			udrSelf.UESubsCollection.Delete(id)
		}
	}
	clearUeSubs()
	t.Cleanup(clearUeSubs)

	var wg sync.WaitGroup
	for u := range ueCount {
		ueId := fmt.Sprintf("imsi-20893010000%04d", u)
		for range perUeRequests {
			wg.Add(1)
			go func(ueId string) {
				defer wg.Done()
				CreateSdmSubscriptionsProcedure(models.SdmSubscription{}, "subscriptionData.contextData.sdmSubscriptions", ueId)
			}(ueId)
		}
	}
	wg.Wait()

	// Every request must be recorded, and each subscription must have a
	// distinct ID: a racing generator would hand out duplicates and lose rows.
	seen := make(map[string]string)
	for u := range ueCount {
		ueId := fmt.Sprintf("imsi-20893010000%04d", u)
		value, ok := udrSelf.UESubsCollection.Load(ueId)
		if !ok {
			t.Fatalf("no UESubsData stored for %s", ueId)
		}
		subs := value.(*udr_context.UESubsData)
		subs.Mtx.RLock()
		got := len(subs.SdmSubscriptions)
		for id := range subs.SdmSubscriptions {
			if prev, dup := seen[id]; dup {
				t.Errorf("subscription ID %s reused by %s and %s", id, prev, ueId)
			}
			seen[id] = ueId
		}
		subs.Mtx.RUnlock()
		if got != perUeRequests {
			t.Errorf("%s: got %d subscriptions, want %d", ueId, got, perUeRequests)
		}
	}
}
