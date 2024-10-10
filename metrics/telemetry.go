// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024 Canonical Ltd.

/*
 *  Metrics package is used to expose the metrics of the UDR service.
 */

package metrics

import (
	"net/http"

	"github.com/omec-project/udr/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// UdrStats captures UDR stats
type UdrStats struct {
	udrSubscriptionData *prometheus.CounterVec
	udrApplicationData  *prometheus.CounterVec
	udrPolicyData       *prometheus.CounterVec
}

var udrStats *UdrStats

func initUdrStats() *UdrStats {
	return &UdrStats{
		udrSubscriptionData: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "udr_subscription_data",
			Help: "Counter of total Subscription data queries",
		}, []string{"query_type", "resource_type", "result"}),
		udrApplicationData: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "udr_application_data",
			Help: "Counter of total Application data queries",
		}, []string{"query_type", "resource_type", "result"}),
		udrPolicyData: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "udr_policy_data",
			Help: "Counter of total Policy data queries",
		}, []string{"query_type", "resource_type", "result"}),
	}
}

func (ps *UdrStats) register() error {
	if err := prometheus.Register(ps.udrSubscriptionData); err != nil {
		return err
	}
	if err := prometheus.Register(ps.udrApplicationData); err != nil {
		return err
	}
	if err := prometheus.Register(ps.udrPolicyData); err != nil {
		return err
	}
	return nil
}

func init() {
	udrStats = initUdrStats()

	if err := udrStats.register(); err != nil {
		logger.InitLog.Errorln("UDR Stats register failed")
	}
}

// InitMetrics initializes UDR metrics
func InitMetrics() {
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logger.InitLog.Errorf("Could not open metrics port: %v", err)
	}
}

// IncrementUdrSubscriptionDataStats increments number of total Subscription data queries
func IncrementUdrSubscriptionDataStats(queryType, resourceType, result string) {
	udrStats.udrSubscriptionData.WithLabelValues(queryType, resourceType, result).Inc()
}

// IncrementUdrApplicationDataStats increments number of total Application data queries
func IncrementUdrApplicationDataStats(queryType, resourceType, result string) {
	udrStats.udrApplicationData.WithLabelValues(queryType, resourceType, result).Inc()
}

// IncrementUdrPolicyDataStats increments number of total Policy data queries
func IncrementUdrPolicyDataStats(queryType, resourceType, result string) {
	udrStats.udrPolicyData.WithLabelValues(queryType, resourceType, result).Inc()
}
