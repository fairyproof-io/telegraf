//go:build !linux
// +build !linux

package cgroup

import (
	"github.com/fairyproof-io/telegraf"
)

func (g *CGroup) Gather(acc telegraf.Accumulator) error {
	return nil
}
