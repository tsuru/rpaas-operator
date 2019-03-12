package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func serviceCreate(c echo.Context) error {
	name := c.FormValue("name")
	if len(name) == 0 {
		return c.String(http.StatusBadRequest, "name is required")
	}
	planName := c.FormValue("plan")
	if len(planName) == 0 {
		return c.String(http.StatusBadRequest, "plan is required")
	}
	team := c.FormValue("team")
	if len(team) == 0 {
		return c.String(http.StatusBadRequest, "team name is required")
	}

	plan, err := getPlan(planName)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return c.String(http.StatusBadRequest, "invalid plan")
		}
		logrus.Error(err)
		return c.String(http.StatusInternalServerError, err.Error())
	}

	instance := &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasInstance",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: NAMESPACE,
			Annotations: map[string]string{
				"user":        c.FormValue("user"),
				"team":        team,
				"description": c.FormValue("description"),
				"eventid":     c.FormValue("eventid"),
			},
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			PlanName: plan.ObjectMeta.Name,
		},
	}
	err = cli.Create(context.TODO(), instance)
	if err != nil {
		logrus.Error(err)
		if k8sErrors.IsAlreadyExists(err) {
			return c.String(http.StatusConflict, name+" instance already exists")
		}
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusCreated)
}

func serviceDelete(c echo.Context) error {
	name := c.Param("instance")
	if len(name) == 0 {
		return c.String(http.StatusBadRequest, "name is required")
	}
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
	err := cli.Delete(context.TODO(), instance)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return c.NoContent(http.StatusNotFound)
		}
		logrus.Error(err)
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.NoContent(http.StatusOK)
}

func servicePlans(c echo.Context) error {
	list := &v1alpha1.RpaasPlanList{}
	err := cli.List(context.TODO(), &client.ListOptions{Namespace: NAMESPACE}, list)
	if err != nil {
		logrus.Error(err)
		return c.JSON(http.StatusInternalServerError, err)
	}
	ret := make([]map[string]string, len(list.Items))
	for i, item := range list.Items {
		ret[i] = map[string]string{
			"name":        item.ObjectMeta.Name,
			"description": "no plan description",
		}
	}
	return c.JSON(http.StatusOK, ret)
}

func getPlan(name string) (*v1alpha1.RpaasPlan, error) {
	instance := &v1alpha1.RpaasPlan{}
	err := cli.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: NAMESPACE}, instance)
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func serviceInfo(c echo.Context) error {
	instance := &v1alpha1.RpaasInstance{}
	err := cli.Get(context.TODO(), types.NamespacedName{Name: c.Param("instance"), Namespace: NAMESPACE}, instance)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return c.NoContent(http.StatusNotFound)
		}
		logrus.Error(err)
		return c.JSON(http.StatusInternalServerError, err)
	}
	replicas := "0"
	if instance.Spec.Replicas != nil {
		replicas = fmt.Sprintf("%d", *instance.Spec.Replicas)
	}
	routes := make([]string, len(instance.Spec.Locations))
	for i, loc := range instance.Spec.Locations {
		routes[i] = loc.Config.Value
	}
	ret := []map[string]string{
		{
			"label": "Address",
			"value": "x.x.x.x",
		},
		{
			"label": "Instances",
			"value": replicas,
		},
		{
			"label": "Routes",
			"value": strings.Join(routes, "\n"),
		},
	}
	return c.JSON(http.StatusOK, ret)
}

func serviceBindApp(c echo.Context) error {
	annotations := map[string]string{
		"app-name": c.FormValue("app-name"),
		"app-host": c.FormValue("app-host"),
		"eventid":  c.FormValue("eventid"),
	}
	name := c.Param("instance")

	instance := &v1alpha1.RpaasBind{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasBind",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   NAMESPACE,
			Annotations: annotations,
		},
	}
	err := cli.Create(context.TODO(), instance)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.NoContent(http.StatusCreated)
}

func serviceUnbindApp(c echo.Context) error {
	name := c.Param("instance")
	instance := &v1alpha1.RpaasBind{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasBind",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: NAMESPACE,
		},
	}
	err := cli.Delete(context.TODO(), instance)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return c.NoContent(http.StatusNotFound)
		}
		logrus.Error(err)
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.NoContent(http.StatusOK)
}
