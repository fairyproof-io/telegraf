package all

import (
	//Blank imports for plugins to register themselves
	_ "github.com/fairyproof-io/telegraf/plugins/aggregators/basicstats"
	_ "github.com/fairyproof-io/telegraf/plugins/aggregators/derivative"
	_ "github.com/fairyproof-io/telegraf/plugins/aggregators/final"
	_ "github.com/fairyproof-io/telegraf/plugins/aggregators/histogram"
	_ "github.com/fairyproof-io/telegraf/plugins/aggregators/merge"
	_ "github.com/fairyproof-io/telegraf/plugins/aggregators/minmax"
	_ "github.com/fairyproof-io/telegraf/plugins/aggregators/quantile"
	_ "github.com/fairyproof-io/telegraf/plugins/aggregators/starlark"
	_ "github.com/fairyproof-io/telegraf/plugins/aggregators/valuecounter"
)
