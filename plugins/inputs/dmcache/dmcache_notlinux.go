//go:build !linux
// +build !linux

package dmcache

import (
	"github.com/fairyproof-io/telegraf"
)

func (c *DMCache) Gather(acc telegraf.Accumulator) error {
	return nil
}

func dmSetupStatus() ([]string, error) {
	return []string{}, nil
}
