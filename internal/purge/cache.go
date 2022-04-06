package purge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-multierror"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func (p *PurgeAPI) cachePurge(c echo.Context) error {
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Request body can't be empty")
	}
	ctx := c.Request().Context()
	var args rpaas.PurgeCacheArgs
	err := c.Bind(&args)
	if err != nil {
		return err
	}
	name := c.Param("instance")
	if len(name) == 0 {
		return c.String(http.StatusBadRequest, "instance is required")
	}

	count, err := p.PurgeCache(ctx, name, args)
	if err != nil && count == 0 {
		return err
	} else if err != nil {
		return c.JSON(http.StatusOK, rpaas.PurgeCacheBulkResult{Path: args.Path, InstancesPurged: count, Error: err.Error()})
	}
	return c.JSON(http.StatusOK, rpaas.PurgeCacheBulkResult{Path: args.Path, InstancesPurged: count})
}

func (p *PurgeAPI) cachePurgeBulk(c echo.Context) error {
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Request body can't be empty")
	}
	ctx := c.Request().Context()

	name := c.Param("instance")
	if len(name) == 0 {
		return c.String(http.StatusBadRequest, "instance is required")
	}

	var argsList []rpaas.PurgeCacheArgs
	if err := json.NewDecoder(c.Request().Body).Decode(&argsList); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	status := http.StatusOK
	var results []rpaas.PurgeCacheBulkResult
	for _, args := range argsList {
		var r rpaas.PurgeCacheBulkResult
		count, err := p.PurgeCache(ctx, name, args)
		if err != nil {
			status = http.StatusInternalServerError
			r = rpaas.PurgeCacheBulkResult{Path: args.Path, InstancesPurged: count, Error: err.Error()}
		} else {
			r = rpaas.PurgeCacheBulkResult{Path: args.Path, InstancesPurged: count}
		}
		results = append(results, r)
	}

	return c.JSON(status, results)
}

func (p *PurgeAPI) PurgeCache(ctx context.Context, name string, args rpaas.PurgeCacheArgs) (int, error) {
	if args.Path == "" {
		return 0, rpaas.ValidationError{Msg: "path is required"}
	}

	pods, port, err := p.lister.ListPods(name)
	if err != nil {
		return 0, rpaas.NotFoundError{Msg: fmt.Sprintf("Failed to find pods: %v", err)}
	}
	logrus.Infof("Found %d pods listening on port %d for instance: %s", len(pods), port, name)

	var purgeErrors error
	purgeCount := 0
	status := false
	for _, pod := range pods {
		if !pod.Running {
			continue
		}
		if status, err = p.cacheManager.PurgeCache(pod.Address, args.Path, port, args.PreservePath); err != nil {
			purgeErrors = multierror.Append(purgeErrors, errors.Wrapf(err, "pod %s:%d failed", pod.Address, port))
			continue
		}
		if status {
			purgeCount++
		}
	}
	return purgeCount, purgeErrors
}
