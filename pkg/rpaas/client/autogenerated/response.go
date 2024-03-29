/*
Reverse Proxy as a Service

The presented API definition (formally called as RPaaS v2 API) is a superset of [Tsuru Service API] and the [legacy RPaaS][RPaaS v1 API] (aka RPaaS v1).  Source code: [github.com/tsuru/rpaas-operator](https://github.com/tsuru/rpaas-operator.git)  [Tsuru Service API]: https://app.swaggerhub.com/apis/tsuru/tsuru-service_api [RPaaS v1 API]: https://raw.githubusercontent.com/tsuru/rpaas/master/rpaas/api.py

API version: v2
Contact: tsuru@g.globo
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package autogenerated

import (
	"net/http"
)

// APIResponse stores the API response returned by the server.
type APIResponse struct {
	*http.Response `json:"-"`
	Message        string `json:"message,omitempty"`
	// Operation is the name of the OpenAPI operation.
	Operation string `json:"operation,omitempty"`
	// RequestURL is the request URL. This value is always available, even if the
	// embedded *http.Response is nil.
	RequestURL string `json:"url,omitempty"`
	// Method is the HTTP method used for the request.  This value is always
	// available, even if the embedded *http.Response is nil.
	Method string `json:"method,omitempty"`
	// Payload holds the contents of the response body (which may be nil or empty).
	// This is provided here as the raw response.Body() reader will have already
	// been drained.
	Payload []byte `json:"-"`
}

// NewAPIResponse returns a new APIResponse object.
func NewAPIResponse(r *http.Response) *APIResponse {

	response := &APIResponse{Response: r}
	return response
}

// NewAPIResponseWithError returns a new APIResponse object with the provided error message.
func NewAPIResponseWithError(errorMessage string) *APIResponse {

	response := &APIResponse{Message: errorMessage}
	return response
}
