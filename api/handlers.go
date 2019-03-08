package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func scale(c echo.Context) error {
	qty := c.FormValue("quantity")
	if len(qty) == 0 {
		return c.String(http.StatusBadRequest, "missing quantity")
	}
	intQty, err := strconv.Atoi(qty)
	if err != nil || intQty <= 0 {
		return c.String(http.StatusBadRequest, "invalid quantity: "+qty)
	}

	name := c.Param("instance")
	instance := &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasInstance",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: NAMESPACE,
		},
	}
	ctx := context.TODO()
	err = cli.Get(ctx, types.NamespacedName{Name: name, Namespace: NAMESPACE}, instance)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return c.NoContent(http.StatusNotFound)
		}
		logrus.Error(err)
		return c.JSON(http.StatusInternalServerError, err)
	}

	int32Qty := int32(intQty)
	instance.Spec.Replicas = &int32Qty
	err = cli.Update(ctx, instance)
	if err != nil {
		logrus.Error(err)
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.NoContent(http.StatusCreated)
}
