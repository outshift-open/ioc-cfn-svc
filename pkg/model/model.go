// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package model

type UserType struct {
	IDPUserID string
	IDPIssuer string
	UUID      string
	Name      string
	Email     string
}

type SessionType struct {
	UserID      string
	AccessToken string
	IDToken     string
}
