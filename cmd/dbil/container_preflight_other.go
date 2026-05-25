//go:build !linux

package main

import "github.com/unkabas/dbil/internal/config"

func prepareContainerRuntime(_ config.DBilConfig) error {
	return nil
}
