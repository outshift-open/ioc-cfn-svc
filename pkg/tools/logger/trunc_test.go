// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrunc(t *testing.T) {
	assert.Equal(t, Trunc("hello", 10), "hello")
	assert.Equal(t, Trunc("hello", 5), "hello")
	assert.Equal(t, Trunc("hello", 2), "he")

	assert.Equal(t, Trunc([]byte("hello"), 10), []byte("hello"))
	assert.Equal(t, Trunc([]byte("hello"), 5), []byte("hello"))
	assert.Equal(t, Trunc([]byte("hello"), 2), []byte("he"))
}
