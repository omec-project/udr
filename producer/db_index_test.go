// SPDX-FileCopyrightText: 2026 Forsway Scandinavia AB
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omec-project/util/mongoapi"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// indexStubDB answers EnsureIndex and nothing else. Every other method of
// DBInterface comes from a nil embedded interface, so a test that reaches one
// panics rather than quietly passing.
type indexStubDB struct {
	DBInterface
	ensured []string
	results []error
}

func (s *indexStubDB) EnsureIndex(_ context.Context, collName string, spec mongoapi.IndexSpec) error {
	s.ensured = append(s.ensured, collName+" "+spec.Name)
	if len(s.results) == 0 {
		return nil
	}
	err := s.results[0]
	s.results = s.results[1:]
	return err
}

func withDBClients(t *testing.T, common, auth DBInterface) {
	t.Helper()
	previousCommon, previousAuth := CommonDBClient, AuthDBClient
	CommonDBClient, AuthDBClient = common, auth
	t.Cleanup(func() { CommonDBClient, AuthDBClient = previousCommon, previousAuth })
}

func withCommonDBClient(t *testing.T, client DBInterface) {
	t.Helper()
	withDBClients(t, client, client)
}

// TestCommonDBIndexesServeEveryFilterOfTheCollectionsTheUdrWrites pins the
// index of each collection to the filters data_repository.go queries and writes
// it by. A filter no index serves scans the whole collection -- on the write
// path too, since a mongoapi write is an update matched on the same filter --
// and nothing at runtime says so.
//
// Order among an index's leading keys is deliberately not asserted: every
// filter here is equality on each field, so (a, b) serves {a, b} exactly as
// well as (b, a) does.
func TestCommonDBIndexesServeEveryFilterOfTheCollectionsTheUdrWrites(t *testing.T) {
	const ueId = "imsi-1"

	filters := map[string][]bson.M{
		SUBSCDATA_AUTHDATA_AUTHSTATUS: {
			{ParamUeId: ueId}, // CreateAuthenticationStatusProcedure, QueryAuthenticationStatusProcedure
		},
		SUBSCDATA_CTXDATA_AMF_3GPPACCESS: {
			{ParamUeId: ueId}, // AmfContext3gpp, CreateAmfContext3gpp, QueryAmfContext3gpp
		},
		SUBSCDATA_CTXDATA_AMF_NON3GPPACCESS: {
			{ParamUeId: ueId}, // AmfContextNon3gpp, CreateAmfContextNon3gpp, QueryAmfContextNon3gpp
		},
		SUBSCDATA_CTXDATA_SMF_REGISTRATION: {
			{ParamUeId: ueId, ParamPduSessionId: int32(5)}, // create, query, delete
			{ParamUeId: ueId}, // QuerySmfRegListProcedure
		},
		SUBSCDATA_CTXDATA_SMSF_3GPPACCESS: {
			{ParamUeId: ueId}, // create, query, delete
		},
		SUBSCDATA_CTXDATA_SMSF_NON3GPPACCESS: {
			{ParamUeId: ueId}, // create, query, delete
		},
		SUBSCDATA_SORDATA: {
			{ParamUeId: ueId}, // CreateSorData, QuerySorData
		},
		POLICYDATA_UES_UEPOLICYSET: {
			{ParamUeId: ueId}, // get, patch, put
		},
		POLICYDATA_UES_OPSPECDATA: {
			{ParamUeId: ueId}, // get, put, JSONPatchExtend
		},
		POLICYDATA_UES_SMDATA_USAGEMONDATA: {
			{ParamUeId: ueId, ParamUsageMonId: "mon-1"}, // put, get, delete
			{ParamUeId: ueId}, // list by ueId
		},
	}

	for collName, collFilters := range filters {
		spec, indexed := commonDBIndexes[collName]
		if !indexed {
			t.Errorf("collection %q is written by the UDR but has no index, so every access to it scans", collName)
			continue
		}
		for _, filter := range collFilters {
			if len(spec.Keys) < len(filter) {
				t.Errorf("index %q has %d key(s), too few to serve a filter on %d field(s)",
					spec.Name, len(spec.Keys), len(filter))
				continue
			}
			leading := map[string]bool{}
			for _, key := range spec.Keys[:len(filter)] {
				leading[key.Key] = true
			}
			for field := range filter {
				if !leading[field] {
					t.Errorf("index %q does not lead with %q, so the filter %v scans %q",
						spec.Name, field, filter, collName)
				}
			}
		}
	}
}

// TestCommonDBIndexesCoverOnlyWhatTheUdrWrites is the other half of the
// ownership rule: an index belongs to the function that writes the collection
// into existence, so the UDR must not claim the collections webconsole writes
// and it only reads.
//
// The list is every provisioned and policy collection data_repository.go reads
// but never creates, not a sample of them: a guard that names some of a set
// passes for the ones it forgot.
func TestCommonDBIndexesCoverOnlyWhatTheUdrWrites(t *testing.T) {
	writtenByWebconsole := []string{
		CollAmData,
		CollSmfSelectionSubscriptionData,
		CollAuthenticationSubscription,
		CollSmsData,
		CollSmsMngData,
		CollTraceData,
		"subscriptionData.provisionedData.smData",
		"policyData.ues.amData",
		"policyData.ues.smData",
	}
	for _, collName := range writtenByWebconsole {
		if _, claimed := commonDBIndexes[collName]; claimed {
			t.Errorf("collection %q is written by webconsole, so its index is webconsole's to create", collName)
		}
	}

	// The UDR reaches these but never creates them: ppData and
	// operatorSpecificData are patched in place, identityData is only read.
	notCreatedHere := []string{
		"subscriptionData.ppData",
		"subscriptionData.operatorSpecificData",
		"subscriptionData.identityData",
	}
	for _, collName := range notCreatedHere {
		if _, claimed := commonDBIndexes[collName]; claimed {
			t.Errorf("collection %q is never written into existence by the UDR, so it is not the UDR's to index", collName)
		}
	}

	// bdtData the UDR does create, and it is deliberately absent: it is keyed
	// by BDT reference, so its size follows the operator's policies rather than
	// the subscriber count this change is about.
	if _, claimed := commonDBIndexes[POLICYDATA_BDTDATA]; claimed {
		t.Errorf("%q is indexed; it is not a per-subscriber collection, so if that is now wanted "+
			"it needs its own justification", POLICYDATA_BDTDATA)
	}
}

// TestCommonDBIndexesAssertNoUniquenessTheDataLacks covers the reason these
// indexes could not be created before layer 1: the only index the library could
// make was unique, and smfRegistrations holds several documents per ueId.
func TestCommonDBIndexesAssertNoUniquenessTheDataLacks(t *testing.T) {
	for collName, spec := range commonDBIndexes {
		if spec.Unique {
			t.Errorf("index %q on %q is unique; none of these is wanted as a constraint, and "+
				"QuerySmfRegListProcedure returns a list per ueId", spec.Name, collName)
		}
		if spec.Name == "" {
			t.Errorf("index on %q has no name, so a later version cannot replace it", collName)
		}
	}
}

func TestEnsureIndexesCreatesOnePerWrittenCollection(t *testing.T) {
	stub := &indexStubDB{}
	withCommonDBClient(t, stub)

	if err := EnsureIndexes(); err != nil {
		t.Fatalf("EnsureIndexes failed: %v", err)
	}
	if len(stub.ensured) != len(commonDBIndexes) {
		t.Fatalf("ensured %d index(es), want %d: %v", len(stub.ensured), len(commonDBIndexes), stub.ensured)
	}
}

func TestEnsureIndexesRetriesAndThenSucceeds(t *testing.T) {
	stub := &indexStubDB{results: []error{errors.New("no primary yet")}}
	withCommonDBClient(t, stub)

	if err := EnsureIndexes(); err != nil {
		t.Fatalf("EnsureIndexes gave up on an error that went away: %v", err)
	}
	if len(stub.ensured) != len(commonDBIndexes)+1 {
		t.Errorf("expected one retry, got %d call(s) for %d index(es)", len(stub.ensured), len(commonDBIndexes))
	}
}

func TestEnsureIndexesReportsWhenThereIsNoConnection(t *testing.T) {
	withCommonDBClient(t, nil)

	err := EnsureIndexes()
	if err == nil {
		t.Fatal("expected an error when ConnectMongo left no client behind")
	}
}

// TestEnsureIndexesReportsWhenOnlyTheAuthDatabaseIsMissing covers the half of
// that which is easy to miss. ConnectMongo sets the two clients separately and
// breaks its loop only when both succeed, so it can time out having connected
// the common database and not the authentication one. Nothing here indexes
// through AuthDBClient, but every authentication handler dereferences it.
func TestEnsureIndexesReportsWhenOnlyTheAuthDatabaseIsMissing(t *testing.T) {
	withDBClients(t, &indexStubDB{}, nil)

	err := EnsureIndexes()
	if err == nil {
		t.Fatal("expected an error when ConnectMongo reached the common database but not the auth one")
	}
}

func TestEnsureIndexWithRetryGivesUpWhenTheBudgetIsSpent(t *testing.T) {
	stub := &indexStubDB{results: []error{errors.New("still no primary")}}

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	err := ensureIndexWithRetry(ctx, stub, CollAmData, mongoapi.IndexSpec{
		Name: "probe",
		Keys: mongoapi.AscendingKeys(ParamUeId),
	})
	if err == nil {
		t.Fatal("expected an error once the budget was spent")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error does not carry the deadline that caused it: %v", err)
	}
}
