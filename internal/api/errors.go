// Package api owns HTTP mappings; business packages do not depend on HTTP.
package api

import (
	"net/http"
	"regexp"

	v1 "normgate.dev/normgate/internal/contracts/v1"
	nerrors "normgate.dev/normgate/internal/errors"
)

var requestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,256}$`)

func ErrorResponse(err error, correlation string) (int, v1.ErrorEnvelope) {
	code := nerrors.CodeOf(err)
	status, message := http.StatusInternalServerError, "Internal failure."
	switch code {
	case nerrors.Validation:
		status, message = http.StatusBadRequest, "Invalid request."
	case nerrors.Authentication:
		status, message = http.StatusUnauthorized, "Authentication required."
	case nerrors.Policy:
		status, message = http.StatusServiceUnavailable, "Policy unavailable."
	case nerrors.Enforcement:
		status, message = http.StatusForbidden, "Operation denied."
	case nerrors.Storage:
		status, message = http.StatusServiceUnavailable, "Storage unavailable."
	case nerrors.Upstream:
		status, message = http.StatusBadGateway, "Upstream unavailable."
	default:
		code = nerrors.Internal
	}
	if !requestID.MatchString(correlation) {
		correlation = "uncorrelated"
	}
	return status, v1.ErrorEnvelope{SchemaVersion: "1.0", RequestId: correlation, Code: string(code), Message: message}
}
