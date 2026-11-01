package health

import (
	"github.com/bidirekt/broker/internal/components"
)

func Register(kernel *components.Components) {
	handler := NewHealthHandler()
	kernel.Server.Get("/health", handler.Handle)
}
