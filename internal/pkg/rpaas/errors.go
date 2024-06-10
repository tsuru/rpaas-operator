// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"errors"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	ErrNoPoolDefined = errors.New("no pool defined")
)

type NotModifiedError struct {
	Msg      string `json:"message"`
	Internal error  `json:"-"`
}

func (NotModifiedError) IsNotModified() bool {
	return true
}

func (e NotModifiedError) Error() string {
	return e.Msg
}

func (e NotModifiedError) Unwrap() error {
	return e.Internal
}

type ValidationError struct {
	Msg      string `json:"message"`
	Internal error  `json:"-"`
}

func (ValidationError) IsValidation() bool {
	return true
}

func (e ValidationError) Error() string {
	return e.Msg
}

func (e ValidationError) Unwrap() error {
	return e.Internal
}

type ConflictError struct {
	Msg      string `json:"message"`
	Internal error  `json:"-"`
}

func (ConflictError) IsConflict() bool {
	return true
}

func (e ConflictError) Error() string {
	return e.Msg
}

func (e ConflictError) Unwrap() error {
	return e.Internal
}

type NotFoundError struct {
	Msg      string `json:"message"`
	Internal error  `json:"-"`
}

func (NotFoundError) IsNotFound() bool {
	return true
}

func (e NotFoundError) Error() string {
	return e.Msg
}

func (e NotFoundError) Unwrap() error {
	return e.Internal
}

func IsNotModifiedError(err error) bool {
	_, ok := err.(interface{ IsNotModified() bool })
	return ok
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
