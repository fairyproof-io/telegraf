package parallel

import "github.com/fairyproof-io/telegraf"

type Parallel interface {
	Enqueue(telegraf.Metric)
	Stop()
}
