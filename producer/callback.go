// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package producer

import (
	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/udr/producer/callback"
)

func PreHandleOnDataChangeNotify(ueId string, resourceId string, patchItems []models.PatchItem,
	origValue interface{}, newValue interface{},
) {
	notifyItems := []models.NotifyItem{}
	changes := []models.ChangeItem{}

	for _, patchItem := range patchItems {
		change := models.NewChangeItem(models.ChangeType(patchItem.GetOp()), patchItem.GetPath())
		if from, ok := patchItem.GetFromOk(); ok {
			change.SetFrom(*from)
		}
		change.SetOrigValue(origValue)
		change.SetNewValue(newValue)
		changes = append(changes, *change)
	}

	notifyItem := models.NewNotifyItem(resourceId, changes)

	notifyItems = append(notifyItems, *notifyItem)

	go callback.SendOnDataChangeNotify(ueId, notifyItems)
}

func PreHandlePolicyDataChangeNotification(ueId string, dataId string, value interface{}) {
	policyDataChangeNotification := models.PolicyDataChangeNotification{}

	if ueId != "" {
		policyDataChangeNotification.SetUeId(ueId)
	}

	switch v := value.(type) {
	case models.AmPolicyData:
		policyDataChangeNotification.SetAmPolicyData(v)
	case models.UePolicySet:
		policyDataChangeNotification.SetUePolicySet(v)
	case models.SmPolicyData:
		policyDataChangeNotification.SetSmPolicyData(v)
	case models.UsageMonData:
		policyDataChangeNotification.SetUsageMonId(dataId)
		policyDataChangeNotification.SetUsageMonData(v)
	case models.SponsorConnectivityData:
		policyDataChangeNotification.SetSponsorId(dataId)
		policyDataChangeNotification.SetSponsorConnectivityData(v)
	case models.BdtData:
		policyDataChangeNotification.SetBdtRefId(dataId)
		policyDataChangeNotification.SetBdtData(v)
	default:
		return
	}

	go callback.SendPolicyDataChangeNotification([]models.PolicyDataChangeNotification{policyDataChangeNotification})
}
