package jolokia2

import (
	"fmt"
	"sync"
	"time"

	"github.com/fairyproof-io/telegraf"
	"github.com/fairyproof-io/telegraf/config"
	"github.com/fairyproof-io/telegraf/plugins/common/tls"
)

type JolokiaAgent struct {
	DefaultFieldPrefix    string
	DefaultFieldSeparator string
	DefaultTagPrefix      string

	URLs            []string `toml:"urls"`
	Username        string
	Password        string
	ResponseTimeout config.Duration `toml:"response_timeout"`

	tls.ClientConfig

	Metrics  []MetricConfig `toml:"metric"`
	gatherer *Gatherer
	clients  []*Client
}

func (ja *JolokiaAgent) SampleConfig() string {
	return `
  # default_tag_prefix      = ""
  # default_field_prefix    = ""
  # default_field_separator = "."

  # Add agents URLs to query
  urls = ["http://localhost:8080/jolokia"]
  # username = ""
  # password = ""
  # response_timeout = "5s"

  ## Optional TLS config
  # tls_ca   = "/var/private/ca.pem"
  # tls_cert = "/var/private/client.pem"
  # tls_key  = "/var/private/client-key.pem"
  # insecure_skip_verify = false

  ## Add metrics to read
  [[inputs.jolokia2_agent.metric]]
    name  = "java_runtime"
    mbean = "java.lang:type=Runtime"
    paths = ["Uptime"]
`
}

func (ja *JolokiaAgent) Description() string {
	return "Read JMX metrics from a Jolokia REST agent endpoint"
}

func (ja *JolokiaAgent) Gather(acc telegraf.Accumulator) error {
	if ja.gatherer == nil {
		ja.gatherer = NewGatherer(ja.createMetrics())
	}

	// Initialize clients once
	if ja.clients == nil {
		ja.clients = make([]*Client, 0, len(ja.URLs))
		for _, url := range ja.URLs {
			client, err := ja.createClient(url)
			if err != nil {
				acc.AddError(fmt.Errorf("unable to create client for %s: %v", url, err))
				continue
			}
			ja.clients = append(ja.clients, client)
		}
	}

	var wg sync.WaitGroup

	for _, client := range ja.clients {
		wg.Add(1)
		go func(client *Client) {
			defer wg.Done()

			err := ja.gatherer.Gather(client, acc)
			if err != nil {
				acc.AddError(fmt.Errorf("unable to gather metrics for %s: %v", client.URL, err))
			}
		}(client)
	}

	wg.Wait()

	return nil
}

func (ja *JolokiaAgent) createMetrics() []Metric {
	var metrics []Metric

	for _, metricConfig := range ja.Metrics {
		metrics = append(metrics, NewMetric(metricConfig,
			ja.DefaultFieldPrefix, ja.DefaultFieldSeparator, ja.DefaultTagPrefix))
	}

	return metrics
}

func (ja *JolokiaAgent) createClient(url string) (*Client, error) {
	return NewClient(url, &ClientConfig{
		Username:        ja.Username,
		Password:        ja.Password,
		ResponseTimeout: time.Duration(ja.ResponseTimeout),
		ClientConfig:    ja.ClientConfig,
	})
}
