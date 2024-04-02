// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package validation

import (
	"context"
	"errors"
	"time"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ rpaas.RpaasManager = &validationManager{}

type validationManager struct {
	rpaas.RpaasManager
	cli client.Client
}

func New(manager rpaas.RpaasManager, cli client.Client) rpaas.RpaasManager {
	return &validationManager{
		RpaasManager: manager,
		cli:          cli,
	}
}

var errNotImplementedYet = errors.New("not implemented yet")

func (v *validationManager) DeleteBlock(ctx context.Context, instanceName, blockName string) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	err = rpaas.NewMutation(&validation.Spec).DeleteBlock(blockName)
	if err != nil {
		return err
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.DeleteBlock(ctx, instanceName, blockName)
}

func (v *validationManager) UpdateBlock(ctx context.Context, instanceName string, block rpaas.ConfigurationBlock) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	err = rpaas.NewMutation(&validation.Spec).UpdateBlock(block)
	if err != nil {
		return err
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.UpdateBlock(ctx, instanceName, block)
}

func (v *validationManager) CreateExtraFiles(ctx context.Context, instanceName string, files ...rpaas.File) error {
	return errNotImplementedYet
}

func (v *validationManager) DeleteExtraFiles(ctx context.Context, instanceName string, filenames ...string) error {
	return errNotImplementedYet
}

func (v *validationManager) UpdateExtraFiles(ctx context.Context, instanceName string, files ...rpaas.File) error {
	return errNotImplementedYet
}

func (v *validationManager) DeleteRoute(ctx context.Context, instanceName, path string) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	err = rpaas.NewMutation(&validation.Spec).DeleteRoute(path)
	if err != nil {
		return err
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.DeleteRoute(ctx, instanceName, path)
}

func (v *validationManager) UpdateRoute(ctx context.Context, instanceName string, route rpaas.Route) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	err = rpaas.NewMutation(&validation.Spec).UpdateRoute(route)
	if err != nil {
		return err
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.UpdateRoute(ctx, instanceName, route)
}

func (v *validationManager) BindApp(ctx context.Context, instanceName string, args rpaas.BindAppArgs) error {
	validation, err := v.validationCRD(ctx, instanceName)
	if err != nil {
		return err
	}

	internalBind := validation.BelongsToCluster(args.AppClusterName)
	err = rpaas.NewMutation(&validation.Spec).BindApp(args, internalBind)
	if err != nil {
		return err
	}

	err = v.waitController(ctx, validation)
	if err != nil {
		return err
	}

	return v.RpaasManager.BindApp(ctx, instanceName, args)
}

func (v *validationManager) UnbindApp(ctx context.Context, instanceName, appName string) error {
	return errNotImplementedYet
}

func (v *validationManager) validationCRD(ctx context.Context, instanceName string) (*v1alpha1.RpaasValidation, error) {
	instance, err := v.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	return &v1alpha1.RpaasValidation{
		ObjectMeta: instance.ObjectMeta,
		Spec:       instance.Spec,
	}, nil
}

func (v *validationManager) waitController(ctx context.Context, validation *v1alpha1.RpaasValidation) error {
	existingValidation := v1alpha1.RpaasValidation{}
	err := v.cli.Get(ctx, client.ObjectKeyFromObject(validation), &existingValidation)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return err
		}
		err = v.cli.Create(ctx, validation, &client.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		validation.ObjectMeta.ResourceVersion = existingValidation.ResourceVersion
		err = v.cli.Update(ctx, validation, &client.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	maxRetries := 30

	for retry := 0; retry < maxRetries; retry++ {
		existingValidation = v1alpha1.RpaasValidation{}
		err = v.cli.Get(ctx, client.ObjectKeyFromObject(validation), &existingValidation)
		if err != nil {
			return err
		}

		if existingValidation.Status.Valid != nil {
			if *existingValidation.Status.Valid {
				return nil
			}

			return &rpaas.ValidationError{Msg: existingValidation.Status.Error}
		}

		time.Sleep(time.Second)
	}

	return &rpaas.ValidationError{Msg: "rpaas-operator timeout"}
}
