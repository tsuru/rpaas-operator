package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func serviceCreate(c echo.Context) error {
	// TODO: add validations (name and plan required, name doesn't exist, plan exists)
	name := c.FormValue("name")
	plan := c.FormValue("plan")
	annotations := map[string]string{
		"user":        c.FormValue("user"),
		"team":        c.FormValue("team"),
		"description": c.FormValue("description"),
		"eventid":     c.FormValue("eventid"),
	}

	instance := &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasInstance",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Annotations: annotations,
		},
		Spec: v1alpha1.RpaasInstanceSpec{
			PlanName: plan,
		},
	}
	client, err := getClient()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	client.Create(context.TODO(), instance)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusCreated)
}

func serviceDelete(c echo.Context) error {
	name := c.Param("instance")
	instance := &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasInstance",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
	client, err := getClient()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	client.Delete(context.TODO(), instance)
	if k8sErrors.IsNotFound(err) {
		return c.NoContent(http.StatusNotFound)
	}
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusOK)
}

func servicePlans(c echo.Context) error {
	list := &v1alpha1.RpaasPlanList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasPlan",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
	}
	client, err := getClient()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	client.List(context.TODO(), &client.ListOptions{Namespace: "default"}, list)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
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

func serviceInfo(c echo.Context) error {
	plan := &v1alpha1.RpaasInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasInstance",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Param("instance"),
			Namespace: "default",
		},
	}
	client, err := getClient()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	client.Get(context.TODO(), client.ObjectKey{Name: c.Param("instance"), Namespace: "default"}, obj)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	replicas := "0"
	if plan.Spec.Replicas != nil {
		replicas = fmt.Sprintf("%d", *plan.Spec.Replicas)
	}
	routes := make([]string, len(plan.Spec.Locations))
	for i, loc := range plan.Spec.Locations {
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
			Namespace:   "default",
			Annotations: annotations,
		},
	}
	client, err := getClient()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	client.Create(context.TODO(), instance)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
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
			Namespace: "default",
		},
	}
	client, err := getClient()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	client.Delete(context.TODO(), instance)
	if k8sErrors.IsNotFound(err) {
		return c.NoContent(http.StatusNotFound)
	}
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusOK)
}

func getClient() (*client.Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	m, err := manager.New(config, manager.Options{})
	if err != nil {
		return nil, err
	}
	return m.GetClient(), nil
}
