// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package util

import (
	"net/http"

	"github.com/omec-project/openapi/v2/models"
)

func ProblemDetailsNotFound(cause string) *models.ProblemDetails {
	title := ""
	switch cause {
	case "USER_NOT_FOUND":
		title = "User not found"
	case "SUBSCRIPTION_NOT_FOUND":
		title = "Subscription not found"
	case "AMFSUBSCRIPTION_NOT_FOUND":
		title = "AMF Subscription not found"
	default:
		title = "Data not found"
	}
	problemDetails := models.NewProblemDetails()
	problemDetails.SetTitle(title)
	problemDetails.SetStatus(http.StatusNotFound)
	problemDetails.SetCause(cause)
	return problemDetails
}

func ProblemDetailsModifyNotAllowed(detail string) *models.ProblemDetails {
	problemDetails := models.NewProblemDetails()
	problemDetails.SetTitle("Modify not allowed")
	problemDetails.SetStatus(http.StatusForbidden)
	problemDetails.SetCause("MODIFY_NOT_ALLOWED")
	problemDetails.SetDetail(detail)
	return problemDetails
}
