package api

import (
	"net/http"

	"github.com/labstack/echo"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func servicePlans(c echo.Context) error {
	list := &v1alpha1.RpaasPlanList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RpaasPlan",
			APIVersion: "extensions.tsuru.io/v1alpha1",
		},
	}
	err := sdk.List("default", list)
	if err != nil {
		return c.String(http.StatusNotFound, "")
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
