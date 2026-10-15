package chafa

// Detect is retained as a no-op so callers can warm the chafa-go path without
// shelling out to a system `chafa` binary.
func Detect() (path string, ok bool) {
	return "", false
}
