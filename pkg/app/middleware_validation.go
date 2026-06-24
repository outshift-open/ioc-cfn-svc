// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"net/http"
	"os"
	"time"

	httpclient "github.com/outshift-open/ioc-cfn-svc/pkg/client/http"
	eh "github.com/outshift-open/ioc-cfn-svc/pkg/tools/easyhttp"
)

func getMgmtURL() string {
	return getEnvOrDefault("MGMT_URL", "http://localhost:9000")
}

func isValidationDisabled() bool {
	return os.Getenv("DISABLE_VALIDATION") == "true"
}

// validateWorkspace calls the management plane to verify that the given workspace
// ID exists. Returns (0, nil) when it exists; returns a non-zero HTTP status and
// error otherwise.
func validateWorkspace(r *http.Request, workspaceID string) (int, error) {
	log := getLogger()

	if isValidationDisabled() {
		log.Debugf("workspace/MAS validation disabled — skipping (workspace=%s)", workspaceID)
		return 0, nil
	}

	mgmtURL := getMgmtURL()
	client := httpclient.New(10 * time.Second)
	headers := map[string]string{"Accept": "application/json"}
	ctx := r.Context()

	wsURL := fmt.Sprintf("%s/api/workspaces/%s", mgmtURL, workspaceID)
	wsResp, err := client.Get(ctx, wsURL, headers)
	if err != nil {
		log.Errorf("failed to reach mgmt-plane validating workspace=%s: %v", workspaceID, err)
		return http.StatusInternalServerError, fmt.Errorf(
			"internal server error while validating workspace '%s'", workspaceID,
		)
	}
	defer wsResp.Body.Close()

	switch wsResp.StatusCode {
	case http.StatusOK:
		return 0, nil
	case http.StatusNotFound:
		return http.StatusNotFound, fmt.Errorf(
			"workspace '%s' not found; visit %s/api/docs or the UI to create one", workspaceID, mgmtURL,
		)
	default:
		log.Errorf("unexpected status %d from mgmt-plane validating workspace=%s", wsResp.StatusCode, workspaceID)
		return http.StatusInternalServerError, fmt.Errorf(
			"internal server error while validating workspace '%s'", workspaceID,
		)
	}
}

// validateWorkspaceAndMas calls the management plane to verify that the given
// workspace ID and MAS ID both exist. Returns (0, nil) when both exist and the
// request may proceed; returns a non-zero HTTP status and error otherwise.
func validateWorkspaceAndMas(r *http.Request, workspaceID, masID string) (int, error) {
	if code, err := validateWorkspace(r, workspaceID); err != nil {
		return code, err
	}
	log := getLogger()

	if isValidationDisabled() {
		log.Debugf("workspace/MAS validation disabled — skipping (masID=%s)", masID)
		return 0, nil
	}

	mgmtURL := getMgmtURL()
	client := httpclient.New(10 * time.Second)
	headers := map[string]string{"Accept": "application/json"}
	ctx := r.Context()

	masURL := fmt.Sprintf("%s/api/workspaces/%s/multi-agentic-systems/%s", mgmtURL, workspaceID, masID)
	masResp, err := client.Get(ctx, masURL, headers)
	if err != nil {
		log.Errorf("failed to reach mgmt-plane validating mas=%s under workspace=%s: %v", masID, workspaceID, err)
		return http.StatusInternalServerError, fmt.Errorf(
			"internal server error while validating MAS '%s' under workspace '%s'", masID, workspaceID,
		)
	}
	defer masResp.Body.Close()

	switch masResp.StatusCode {
	case http.StatusOK:
		return 0, nil
	case http.StatusNotFound:
		return http.StatusNotFound, fmt.Errorf(
			"MAS '%s' not found under workspace '%s'; visit %s/api/docs or the UI to create one",
			masID, workspaceID, mgmtURL,
		)
	default:
		log.Errorf("unexpected status %d from mgmt-plane validating mas=%s under workspace=%s", masResp.StatusCode, masID, workspaceID)
		return http.StatusInternalServerError, fmt.Errorf(
			"internal server error while validating MAS '%s' under workspace '%s'", masID, workspaceID,
		)
	}
}

// withWorkspaceValidation wraps an easyHandler to validate that the {workspaceId}
// path parameter refers to an existing workspace in the management plane.
func withWorkspaceValidation(inner func(http.ResponseWriter, *http.Request) (int, error)) func(http.ResponseWriter, *http.Request) (int, error) {
	return func(w http.ResponseWriter, r *http.Request) (int, error) {
		workspaceID := eh.PathParam(r, "workspaceId")

		if code, err := validateWorkspace(r, workspaceID); err != nil {
			return code, err
		}

		return inner(w, r)
	}
}

// withWorkspaceAndMasValidation wraps an easyHandler to validate that the
// {workspaceId} and {masId} path parameters refer to existing resources in the
// management plane before delegating to the inner handler.
func withWorkspaceAndMasValidation(inner func(http.ResponseWriter, *http.Request) (int, error)) func(http.ResponseWriter, *http.Request) (int, error) {
	return func(w http.ResponseWriter, r *http.Request) (int, error) {
		workspaceID := eh.PathParam(r, "workspaceId")
		masID := eh.PathParam(r, "masId")

		if code, err := validateWorkspaceAndMas(r, workspaceID, masID); err != nil {
			return code, err
		}

		return inner(w, r)
	}
}
