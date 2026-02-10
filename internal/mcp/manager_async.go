package mcp

import (
	"context"
	"time"
)

// CallWithTimeout wraps Call with a timeout.
func (m *Manager) CallWithTimeout(ctx context.Context, name, method string, params interface{}, timeout time.Duration) (interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	result, err := m.Call(name, method, params)
	if err != nil {
		return nil, err
	}
	return result, nil
}
