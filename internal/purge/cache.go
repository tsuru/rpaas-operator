package purge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

func (p *purge) cachePurge(c echo.Context) error {
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
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, fmt.Sprintf("Object purged on %d servers", count))
}

func (p *purge) cachePurgeBulk(c echo.Context) error {
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
			r = rpaas.PurgeCacheBulkResult{Path: args.Path, Error: err.Error()}
		} else {
			r = rpaas.PurgeCacheBulkResult{Path: args.Path, InstancesPurged: count}
		}
		results = append(results, r)
	}

	return c.JSON(status, results)
}

func (p *purge) PurgeCache(ctx context.Context, name string, args rpaas.PurgeCacheArgs) (int, error) {
	if args.Path == "" {
		return 0, rpaas.ValidationError{Msg: "path is required"}
	}
	pods, port, err := p.watcher.ListPods(name)
	if err != nil {
		return 0, rpaas.NotFoundError{Msg: fmt.Sprintf("Failed to find pods: %v", err)}
	}
	// ToDo: better error handling (accumulate errors?)
	purgeCount := 0
	for _, pod := range pods {
		if !pod.Running {
			continue
		}
		if err = p.cacheManager.PurgeCache(pod.Address, args.Path, port, args.PreservePath); err != nil {
			continue
		}
		purgeCount++
	}
	return purgeCount, err
}
