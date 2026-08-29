package app

// RenderTestRequest configures poke test (renderer drag race + visual compare).
type RenderTestRequest struct {
	Target     string
	Iterations int
	SkipVisual bool
}
