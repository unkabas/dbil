package dockerapi

import "os"

// getEnv exists only so envDockerHost is overridable in tests without
// pulling in os in the test file directly.
func getEnv(k string) string { return os.Getenv(k) }
