package swap

import (
	"fmt"

	"github.com/fairyproof-io/telegraf"
	"github.com/fairyproof-io/telegraf/plugins/inputs"
	"github.com/fairyproof-io/telegraf/plugins/inputs/system"
)

type SwapStats struct {
	ps system.PS
}

func (ss *SwapStats) Description() string {
	return "Read metrics about swap memory usage"
}

func (ss *SwapStats) SampleConfig() string { return "" }

func (ss *SwapStats) Gather(acc telegraf.Accumulator) error {
	swap, err := ss.ps.SwapStat()
	if err != nil {
		return fmt.Errorf("error getting swap memory info: %s", err)
	}

	fieldsG := map[string]interface{}{
		"total":        swap.Total,
		"used":         swap.Used,
		"free":         swap.Free,
		"used_percent": swap.UsedPercent,
	}
	fieldsC := map[string]interface{}{
		"in":  swap.Sin,
		"out": swap.Sout,
	}
	acc.AddGauge("swap", fieldsG, nil)
	acc.AddCounter("swap", fieldsC, nil)

	return nil
}

func init() {
	ps := system.NewSystemPS()
	inputs.Add("swap", func() telegraf.Input {
		return &SwapStats{ps: ps}
	})
}
