package api_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"normgate.dev/normgate/internal/api"
	"normgate.dev/normgate/internal/contracts"
	nerrors "normgate.dev/normgate/internal/errors"
	"normgate.dev/normgate/internal/policy"
)

func TestNGF006ErrorEnvelope(t *testing.T) {
	for _, tc := range []struct {
		code   nerrors.Code
		status int
	}{{nerrors.Validation, 400}, {nerrors.Authentication, 401}, {nerrors.Policy, 503}, {nerrors.Enforcement, 403}, {nerrors.Storage, 503}, {nerrors.Upstream, 502}, {nerrors.Internal, 500}, {"unknown", 500}} {
		err := nerrors.New(tc.code, errors.New("secret diagnostic"))
		if strings.Contains(err.Error(), "secret") {
			t.Fatal("error string leaked its cause")
		}
		status, envelope := api.ErrorResponse(err, "request-1")
		if status != tc.status {
			t.Fatalf("NG-F006: %s status %d", tc.code, status)
		}
		data, e := json.Marshal(envelope)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(data), "secret") {
			t.Fatal("public secret leak")
		}
		if e := contracts.Validate("error-envelope", data); e != nil {
			t.Fatal(e)
		}
		if !errors.Is(err, err.Unwrap()) {
			t.Fatal("cause not retained internally")
		}
	}
	status, envelope := api.ErrorResponse(errors.New("secret"), "bad\r\nheader")
	if status != 500 || envelope.RequestId != "uncorrelated" {
		t.Fatal("unsafe request correlation")
	}
}

func TestNGF006Snapshot(t *testing.T) {
	_, envelope := api.ErrorResponse(nerrors.New(nerrors.Validation, nil), "request-1")
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"schema_version":"1.0","request_id":"request-1","code":"ng.validation.invalid_input","message":"Invalid request."}`
	if string(data) != want {
		t.Fatalf("NG-F006: public error changed: %s", data)
	}
}

func TestPolicyErrorsArePublicAndRedacted(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{policy.Failure("invalid_event"), 400, "ng.validation.invalid_input"},
		{&policy.Error{Code: "ng.policy.compile_error", File: "private-source.rego", Row: 3, Column: 1}, 503, "ng.policy.unavailable"},
	} {
		status, envelope := api.ErrorResponse(tc.err, "request")
		if status != tc.status || envelope.Code != tc.code || strings.Contains(envelope.Message, "private") {
			t.Fatal(status, envelope)
		}
	}
}
