package outputs

import "github.com/fairyproof-io/telegraf"

// Deprecations lists the deprecated plugins
var Deprecations = map[string]telegraf.DeprecationInfo{
	"riemann_legacy": {
		Since:  "1.3.0",
		Notice: "use 'outputs.riemann' instead (see https://github.com/fairyproof-io/telegraf/issues/1878)",
	},
}
