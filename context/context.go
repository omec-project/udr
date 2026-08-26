// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package context

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/omec-project/openapi/v2/models"
)

var udrContext = UDRContext{}

type subsId = string

type UDRServiceType int

const (
	NUDR_DR UDRServiceType = iota
)

func init() {
	UDR_Self().Name = "udr"
	UDR_Self().EeSubscriptionIDGenerator.Store(1)
	UDR_Self().SubscriptionDataSubscriptionIDGenerator = 1
	UDR_Self().PolicyDataSubscriptionIDGenerator = 1
	UDR_Self().SubscriptionDataSubscriptions = make(map[subsId]*models.SubscriptionDataSubscriptions)
	UDR_Self().PolicyDataSubscriptions = make(map[subsId]*models.PolicyDataSubscription)
}

type UDRContext struct {
	Name                                    string
	UriScheme                               models.UriScheme
	BindingIPv4                             string
	Key                                     string
	PEM                                     string
	RegisterIPv4                            string // IP register to NRF
	HttpIPv6Address                         string
	NfId                                    string
	NrfUri                                  string
	SubscriptionDataSubscriptions           map[subsId]*models.SubscriptionDataSubscriptions
	PolicyDataSubscriptions                 map[subsId]*models.PolicyDataSubscription
	UESubsCollection                        sync.Map // map[ueId]*UESubsData
	UEGroupCollection                       sync.Map // map[ueGroupId]*UEGroupSubsData
	mtx                                     sync.RWMutex
	SBIPort                                 int
	EeSubscriptionIDGenerator               atomic.Int64
	SdmSubscriptionIDGenerator              atomic.Int64
	PolicyDataSubscriptionIDGenerator       int
	SubscriptionDataSubscriptionIDGenerator int
	appDataInfluDataSubscriptionIdGenerator uint64
}

// UESubsData holds the per-UE subscription maps.
//
// Mtx guards both maps. The UDM creates an SDM subscription per registration,
// one goroutine per in-flight registration, so unsynchronised access aborts the
// process with "concurrent map writes"; the EE subscription paths are reached
// from their own handlers and are no different.
//
// Read paths must hold RLock across the map lookup *and* the use of the value
// pointer, not just the lookup: the entry can otherwise be replaced while the
// caller is dereferencing it.
type UESubsData struct {
	EeSubscriptionCollection map[subsId]*EeSubscriptionCollection
	SdmSubscriptions         map[subsId]*models.SdmSubscription
	Mtx                      sync.RWMutex
}

// Mtx guards EeSubscriptions. Keyed by group rather than by UE, so it needs its
// own lock rather than borrowing the per-UE one.
type UEGroupSubsData struct {
	EeSubscriptions map[subsId]*models.EeSubscription
	Mtx             sync.RWMutex
}

type EeSubscriptionCollection struct {
	EeSubscriptions      *models.EeSubscription
	AmfSubscriptionInfos []models.AmfSubscriptionInfo
}

func (context *UDRContext) GetIPv4GroupUri(udrServiceType UDRServiceType) string {
	var serviceUri string

	switch udrServiceType {
	case NUDR_DR:
		serviceUri = "/nudr-dr/v2"
	default:
		serviceUri = ""
	}

	return fmt.Sprintf("%s://%s:%d%s", context.UriScheme, context.RegisterIPv4, context.SBIPort, serviceUri)
}

// Create new UDR context
func UDR_Self() *UDRContext {
	return &udrContext
}

func (context *UDRContext) NewAppDataInfluDataSubscriptionID() uint64 {
	context.mtx.Lock()
	defer context.mtx.Unlock()
	context.appDataInfluDataSubscriptionIdGenerator++
	return context.appDataInfluDataSubscriptionIdGenerator
}
