package aws

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/aws/smithy-go/middleware"
)

type RequestCount struct {
	Service   string
	Operation string
	Count     uint64
}

type RequestSnapshot struct {
	Total            uint64
	EstimatedCostUSD float64
	Requests         []RequestCount
}

type requestObserver struct {
	mu     sync.Mutex
	counts map[string]uint64
}

func newRequestObserver() *requestObserver {
	return &requestObserver{counts: make(map[string]uint64)}
}

func (o *requestObserver) middleware(stack *middleware.Stack) error {
	return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("e9sRequestObserver", func(
		ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler,
	) (middleware.InitializeOutput, middleware.Metadata, error) {
		service := middleware.GetServiceID(ctx)
		operation := middleware.GetOperationName(ctx)
		o.mu.Lock()
		o.counts[service+"\x00"+operation]++
		o.mu.Unlock()
		return next.HandleInitialize(ctx, in)
	}), middleware.Before)
}

func (o *requestObserver) snapshot() RequestSnapshot {
	o.mu.Lock()
	defer o.mu.Unlock()
	result := RequestSnapshot{Requests: make([]RequestCount, 0, len(o.counts))}
	for key, count := range o.counts {
		service, operation, _ := strings.Cut(key, "\x00")
		result.Total += count
		result.EstimatedCostUSD += estimatedRequestCost(service, operation) * float64(count)
		result.Requests = append(result.Requests, RequestCount{Service: service, Operation: operation, Count: count})
	}
	sort.Slice(result.Requests, func(i, j int) bool {
		if result.Requests[i].Service == result.Requests[j].Service {
			return result.Requests[i].Operation < result.Requests[j].Operation
		}
		return result.Requests[i].Service < result.Requests[j].Service
	})
	return result
}

func estimatedRequestCost(service, operation string) float64 {
	switch strings.ToLower(service) {
	case "cost explorer", "costexplorer", "ce":
		return 0.01
	case "secrets manager", "secretsmanager":
		return 0.05 / 10000
	default:
		return 0
	}
}
