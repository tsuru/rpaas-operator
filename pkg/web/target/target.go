package target

import (
	"context"
	"net/http"

	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
)

type Factory interface {
	Manager(ctx context.Context, header http.Header) (rpaas.RpaasManager, error)
}
