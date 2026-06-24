// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package logger

// /////////////////////////////////////////////////////////////////////////////
// adapter to use Logger with github.com/go-logr/logr interface
// /////////////////////////////////////////////////////////////////////////////
type GoLogr struct {
	l Logger
}

func WrapGoLogr(l Logger) *GoLogr {
	return &GoLogr{l: l}
}

func (gl *GoLogr) Info(msg string, keysAndValues ...interface{}) {
	gl.l.Info(msg, keysAndValues)
}

func (gl *GoLogr) Error(err error, msg string, keysAndValues ...interface{}) {
	gl.l.Error(err, msg, keysAndValues)
}
