// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package logger

type Printable interface {
	string | []byte
}

func Trunc[P Printable](p P, lim int) P {
	if len(p) <= lim {
		return p
	}
	return p[:lim] // + P("...")
}
