// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"github.com/pkg/errors"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	ErrNoPoolDefined = errors.New("No pool defined")
)

type ValidationError struct {
	Msg string
}

func (ValidationError) IsValidation() bool {
	return true
}
func (e ValidationError) Error() string {
	return e.Msg
}

type ConflictError struct {
	Msg string
}

func (ConflictError) IsConflict() bool {
	return true
}
func (e ConflictError) Error() string {
	return e.Msg
}

type NotFoundError struct {
	Msg string
}

func (NotFoundError) IsNotFound() bool {
	return true
}
func (e NotFoundError) Error() string {
	return e.Msg
}

func IsValidationError(err error) bool {
	if vErr, ok := err.(interface {
		IsValidation() bool
	}); ok {
		return vErr.IsValidation()
	}
	return k8sErrors.IsBadRequest(err)
}

func IsConflictError(err error) bool {
	if vErr, ok := err.(interface {
		IsConflict() bool
	}); ok {
		return vErr.IsConflict()
	}
	return k8sErrors.IsConflict(err)
}

func IsNotFoundError(err error) bool {
	if vErr, ok := err.(interface {
		IsNotFound() bool
	}); ok {
		return vErr.IsNotFound()
	}
	return k8sErrors.IsNotFound(err)
}
