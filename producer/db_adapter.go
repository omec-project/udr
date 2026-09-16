// SPDX-FileCopyrightText: 2024 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
package producer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/omec-project/udr/logger"
	"github.com/omec-project/util/mongoapi"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type DBInterface interface {
	RestfulAPIGetOne(collName string, filter bson.M) (map[string]interface{}, error)
	RestfulAPIGetMany(collName string, filter bson.M) ([]map[string]interface{}, error)
	RestfulAPIPutOneTimeout(collName string, filter bson.M, putData map[string]interface{}, timeout int32, timeField string) bool
	RestfulAPIPutOne(collName string, filter bson.M, putData map[string]interface{}) (bool, error)
	RestfulAPIPutOneNotUpdate(collName string, filter bson.M, putData map[string]interface{}) (bool, error)
	RestfulAPIPutMany(collName string, filterArray []bson.M, putDataArray []map[string]interface{}) error
	RestfulAPIDeleteOne(collName string, filter bson.M) error
	RestfulAPIDeleteMany(collName string, filter bson.M) error
	RestfulAPIMergePatch(collName string, filter bson.M, patchData map[string]interface{}) error
	RestfulAPIJSONPatch(collName string, filter bson.M, patchJSON []byte) error
	RestfulAPIJSONPatchExtend(collName string, filter bson.M, patchJSON []byte, dataName string) error
	RestfulAPIPost(collName string, filter bson.M, postData map[string]interface{}) (bool, error)
	RestfulAPIPostMany(collName string, filter bson.M, postDataArray []interface{}) error
	EnsureIndex(ctx context.Context, collName string, spec mongoapi.IndexSpec) error
}

var (
	CommonDBClient DBInterface
	AuthDBClient   DBInterface
)

// Set CommonDBClient
func setCommonDBClient(url string, dbname string) error {
	mClient, errConnect := mongoapi.NewMongoClient(url, dbname)
	if mClient != nil && mClient.Client != nil {
		CommonDBClient = newCachedDBClient(mClient)
	}
	return errConnect
}

// Set AuthDBClient
func setAuthDBClient(authurl string, authkeysdbname string) error {
	mClient, errConnect := mongoapi.NewMongoClient(authurl, authkeysdbname)
	if mClient != nil && mClient.Client != nil {
		AuthDBClient = newCachedDBClient(mClient)
	}
	return errConnect
}

func ConnectMongo(url string, dbname string, authurl string, authkeysdbname string) {
	// Connect to MongoDB
	ticker := time.NewTicker(2 * time.Second)
	defer func() { ticker.Stop() }()
	timer := time.After(180 * time.Second)
ConnectMongo:
	for {
		commonDbErr := setCommonDBClient(url, dbname)
		authDbErr := setAuthDBClient(authurl, authkeysdbname)
		if commonDbErr == nil && authDbErr == nil {
			break ConnectMongo
		}
		select {
		case <-ticker.C:
			continue
		case <-timer:
			logger.DataRepoLog.Errorln("Timed out while connecting to MongoDB in 3 minutes.")
			return
		}
	}

	logger.DataRepoLog.Infoln("Connected to MongoDB.")
}

const (
	// indexEnsureBudget bounds ensureIndexes as a whole. It is generous because
	// the usual reason to need it is a MongoDB that elected a primary later than
	// the UDR started, and giving up before that happens only restarts the same
	// wait.
	indexEnsureBudget = 5 * time.Minute
	// indexEnsureAttemptTimeout bounds one EnsureIndex call, which builds the
	// index and then reads the collection's indexes back to confirm it.
	indexEnsureAttemptTimeout = 30 * time.Second
	indexEnsureInitialBackoff = 1 * time.Second
	indexEnsureMaxBackoff     = 30 * time.Second
)

// commonDBIndexes are the indexes the UDR creates, one per collection it writes
// into existence with a per-subscriber filter.
//
// It deliberately does not cover the provisioned, policy and
// authentication-subscription collections the UDR only reads: those are created
// by webconsole, which is their only writer, and an index belongs with the
// function that makes the collection rather than with one of its readers. Nor
// the ones the UDR only patches -- subscriptionData.ppData and
// subscriptionData.operatorSpecificData are modified in place but never created
// here. policyData.bdtData is written by the UDR and is left out for a different
// reason: it is keyed by BDT reference rather than by subscriber, so its size is
// a function of the operator's policies and not of how many UEs are provisioned.
//
// None of them is unique. The uniqueness is not wanted -- these exist so a
// lookup seeks -- and smfRegistrations could not carry it anyway, since
// QuerySmfRegListProcedure filters on ueId alone and expects a list back.
//
// Each key is also the filter of the collection's writes, not only its reads.
// A mongoapi write is an UpdateOne matched on the same filter, so on an
// unindexed collection it pays the same full scan the read does; most of these
// collections are written more often than they are queried.
var commonDBIndexes = map[string]mongoapi.IndexSpec{
	SUBSCDATA_AUTHDATA_AUTHSTATUS: {
		Name: "authenticationStatusByUeId",
		Keys: mongoapi.AscendingKeys(ParamUeId),
	},
	SUBSCDATA_CTXDATA_AMF_3GPPACCESS: {
		Name: "amf3gppAccessByUeId",
		Keys: mongoapi.AscendingKeys(ParamUeId),
	},
	SUBSCDATA_CTXDATA_AMF_NON3GPPACCESS: {
		Name: "amfNon3gppAccessByUeId",
		Keys: mongoapi.AscendingKeys(ParamUeId),
	},
	SUBSCDATA_CTXDATA_SMF_REGISTRATION: {
		// One index for four call sites: the query, the create and the delete
		// all filter (ueId, pduSessionId), and QuerySmfRegListProcedure filters
		// ueId alone, which the leading key already serves.
		Name: "smfRegistrationsByUeIdAndPduSessionId",
		Keys: mongoapi.AscendingKeys(ParamUeId, ParamPduSessionId),
	},
	SUBSCDATA_CTXDATA_SMSF_3GPPACCESS: {
		Name: "smsf3gppAccessByUeId",
		Keys: mongoapi.AscendingKeys(ParamUeId),
	},
	SUBSCDATA_CTXDATA_SMSF_NON3GPPACCESS: {
		Name: "smsfNon3gppAccessByUeId",
		Keys: mongoapi.AscendingKeys(ParamUeId),
	},
	SUBSCDATA_SORDATA: {
		Name: "sorDataByUeId",
		Keys: mongoapi.AscendingKeys(ParamUeId),
	},
	POLICYDATA_UES_UEPOLICYSET: {
		Name: "uePolicySetByUeId",
		Keys: mongoapi.AscendingKeys(ParamUeId),
	},
	POLICYDATA_UES_OPSPECDATA: {
		Name: "policyOperatorSpecificDataByUeId",
		Keys: mongoapi.AscendingKeys(ParamUeId),
	},
	POLICYDATA_UES_SMDATA_USAGEMONDATA: {
		// The put, the get and the delete filter (ueId, usageMonId); the
		// list-by-ueId and the limitId merge patch are served by the leading
		// key. limitId is deliberately not a second index -- it appears in one
		// filter, always alongside ueId.
		Name: "usageMonDataByUeIdAndUsageMonId",
		Keys: mongoapi.AscendingKeys(ParamUeId, ParamUsageMonId),
	},
}

// EnsureIndexes makes every collection the UDR writes carry its index before
// the UDR serves, and does not return without them.
//
// Creating an index is a write, so it fails while the replica set has no
// writable primary yet, which is routine when the UDR and MongoDB start
// together -- hence the retry rather than a single log line. Continuing without
// the indexes is what it must not do: every read and every write of these
// collections degrades to a scan whose cost grows with the number of
// subscribers provisioned, and nothing at runtime says so.
func EnsureIndexes() error {
	// ConnectMongo gives up after three minutes and returns, leaving whichever
	// client it could not reach unset rather than reporting it. Both are
	// checked, not only the one this function indexes through: it sets the two
	// separately and breaks its loop only when both succeed, so it can return
	// having connected the common database and not the authentication one --
	// and every authentication handler dereferences AuthDBClient. Saying so
	// here is the difference between a named startup failure and a nil
	// dereference on the first authentication.
	if CommonDBClient == nil || AuthDBClient == nil {
		return errors.New("no connection to MongoDB, so no index could be created")
	}

	ctx, cancel := context.WithTimeout(context.Background(), indexEnsureBudget)
	defer cancel()

	for _, collName := range sortedIndexedCollections() {
		spec := commonDBIndexes[collName]
		if err := ensureIndexWithRetry(ctx, CommonDBClient, collName, spec); err != nil {
			return err
		}
		logger.DataRepoLog.Infof("index %q is present on collection %q", spec.Name, collName)
	}
	return nil
}

// sortedIndexedCollections gives commonDBIndexes a stable order, so a failure
// names the same collection on every run.
func sortedIndexedCollections() []string {
	names := make([]string, 0, len(commonDBIndexes))
	for collName := range commonDBIndexes {
		names = append(names, collName)
	}
	sort.Strings(names)
	return names
}

// ensureIndexWithRetry calls EnsureIndex until it succeeds or ctx expires,
// backing off between attempts.
func ensureIndexWithRetry(ctx context.Context, client DBInterface, collName string, spec mongoapi.IndexSpec) error {
	backoff := indexEnsureInitialBackoff
	for attempt := 1; ; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, indexEnsureAttemptTimeout)
		err := client.EnsureIndex(attemptCtx, collName, spec)
		cancel()
		if err == nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("could not ensure index %q on collection %q after %d attempts: %w (last error: %v)",
				spec.Name, collName, attempt, ctxErr, err)
		}
		logger.DataRepoLog.Warnf("attempt %d to ensure index %q on collection %q failed, retrying in %s: %v",
			attempt, spec.Name, collName, backoff, err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("could not ensure index %q on collection %q after %d attempts: %w (last error: %v)",
				spec.Name, collName, attempt, ctx.Err(), err)
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, indexEnsureMaxBackoff)
	}
}
