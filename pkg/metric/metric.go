// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Counter:   A counter metric always increases
// Gauge:     A gauge metric can increase or decrease
// Histogram: A histogram metric can increase or descrease to track sampled
//            observations over time


func Handler() http.Handler {
	return promhttp.Handler()
}
