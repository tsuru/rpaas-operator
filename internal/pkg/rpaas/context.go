// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import "context"

type contextKey struct{}

var rpaasManagerKey = contextKey{}

func ContextWithRpaasManager(ctx context.Context, manager RpaasManager) context.Context {
	return context.WithValue(ctx, rpaasManagerKey, manager)
}

func RpaasManagerFromContext(ctx context.Context) RpaasManager {
	val := ctx.Value(rpaasManagerKey)
	if manager, ok := val.(RpaasManager); ok {
		return manager
	}

	return nil
}
