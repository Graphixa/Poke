package cache

import "context"

type interactiveKey struct{}

// WithInteractivePriority marks a context as user-facing (Enter/detail).
// Background warmers throttle so interactive fetches are not starved.
func WithInteractivePriority(ctx context.Context) context.Context {
	return context.WithValue(ctx, interactiveKey{}, true)
}

func interactivePriority(ctx context.Context) bool {
	v, _ := ctx.Value(interactiveKey{}).(bool)
	return v
}
