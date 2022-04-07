package contracts_verified

import (
	"fmt"
	"github.com/fairyproof-io/telegraf"
	"github.com/fairyproof-io/telegraf/plugins/inputs"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/debug"
)

type Network struct {
	Name       string `toml:"name"`
	BrowserUrl string `toml:"browser_url"`
}

// ContractsVerified struct should be named the same as the Plugin
type ContractsVerified struct {
	Networks     []Network `toml:"networks"`
	collector    *colly.Collector
	networkIndex int
	page         int
	Log          telegraf.Logger `toml:"-"`
}

// Usually the default (example) configuration is contained in this constant.
// Please use '## '' to denote comments and '# ' to specify default settings and start each line with two spaces.
const sampleConfig = `
 [[inputs.contracts_verified.networks]]
    name        = "eth"
    browser_url = "https://etherscan.io"
 [[inputs.contracts_verified.networks]]
    name        = "bsc"
    browser_url = "https://bscscan.com"
`

// Description will appear directly above the plugin definition in the config file
func (m *ContractsVerified) Description() string {
	return `This is an contracts verified plugin`
}

// SampleConfig will populate the sample configuration portion of the plugin's configuration
func (m *ContractsVerified) SampleConfig() string {
	return sampleConfig
}

// Init can be implemented to do one-time processing stuff like initializing variables
func (m *ContractsVerified) Init() error {
	m.Log.Debug("ContractsVerified init")
	// Check your options according to your requirements
	if len(m.Networks) == 0 {
		return fmt.Errorf("networks cannot be empty")
	}

	m.collector = colly.NewCollector(colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.163 Safari/537.36"),
		colly.AllowURLRevisit(),
		colly.MaxDepth(1),
		colly.Debugger(&debug.LogDebugger{}))
	m.collector.OnRequest(func(r *colly.Request) {
		m.Log.Debugf("Visiting %s", r.URL.String())
	})

	return nil
}

// Gather defines what data the plugin will gather.
func (m *ContractsVerified) Gather(acc telegraf.Accumulator) error {
	m.Log.Debug("ContractsVerified Gather")

	m.networkIndex = 0
	m.page = 1

	m.collector.OnHTML("tbody", func(e *colly.HTMLElement) {
		var network = m.Networks[m.networkIndex]
		e.ForEach("tr", func(i int, item *colly.HTMLElement) {

			fields := map[string]interface{}{"network": network.Name}

			item.ForEach("td", func(i int, element *colly.HTMLElement) {
				switch i {
				case 0:

					fields["address"] = element.Text
				case 1:
					fields["contract_name"] = element.Text
				case 2:
					fields["compiler"] = element.Text
				case 3:
					fields["version"] = element.Text
				case 4:
					fields["txns"] = element.Text
				default:
					break
				}
			})
			m.Log.Debugf("%s", fields)
			acc.AddFields("ContractsVerified", fields, nil)

		})

		m.page++
		if m.page > 5 {
			if m.nextNetwork() == false {
				return
			}
		}
		m.Visit()

	})
	m.Visit()

	return nil
}

func (m *ContractsVerified) nextNetwork() bool {
	if m.networkIndex >= len(m.Networks)-1 {
		return false
	}

	m.networkIndex++
	m.page = 1

	return true
}

func (m *ContractsVerified) Visit() {
	if m.networkIndex >= len(m.Networks) {
		return
	}

	var network = m.Networks[m.networkIndex]
	url := fmt.Sprintf("%s/contractsVerified/%d?ps=100", network.BrowserUrl, m.page)
	err := m.collector.Visit(url)
	if err != nil {
		m.Log.Errorf("Visit Visiting %s", err.Error())
	}
}

// Register the plugin
func init() {
	inputs.Add("contracts_verified", func() telegraf.Input {
		return &ContractsVerified{}
	})
}
