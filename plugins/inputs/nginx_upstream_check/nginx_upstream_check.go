package nginx_upstream_check

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/fairyproof-io/telegraf"
	"github.com/fairyproof-io/telegraf/config"
	"github.com/fairyproof-io/telegraf/plugins/common/tls"
	"github.com/fairyproof-io/telegraf/plugins/inputs"
)

const sampleConfig = `
  ## An URL where Nginx Upstream check module is enabled
  ## It should be set to return a JSON formatted response
  url = "http://127.0.0.1/status?format=json"

  ## HTTP method
  # method = "GET"

  ## Optional HTTP headers
  # headers = {"X-Special-Header" = "Special-Value"}

  ## Override HTTP "Host" header
  # host_header = "check.example.com"

  ## Timeout for HTTP requests
  timeout = "5s"

  ## Optional HTTP Basic Auth credentials
  # username = "username"
  # password = "pa$$word"

  ## Optional TLS Config
  # tls_ca = "/etc/telegraf/ca.pem"
  # tls_cert = "/etc/telegraf/cert.pem"
  # tls_key = "/etc/telegraf/key.pem"
  ## Use TLS but skip chain & host verification
  # insecure_skip_verify = false
`

const description = "Read nginx_upstream_check module status information (https://github.com/yaoweibin/nginx_upstream_check_module)"

type NginxUpstreamCheck struct {
	URL string `toml:"url"`

	Username   string            `toml:"username"`
	Password   string            `toml:"password"`
	Method     string            `toml:"method"`
	Headers    map[string]string `toml:"headers"`
	HostHeader string            `toml:"host_header"`
	Timeout    config.Duration   `toml:"timeout"`

	tls.ClientConfig
	client *http.Client
}

func NewNginxUpstreamCheck() *NginxUpstreamCheck {
	return &NginxUpstreamCheck{
		URL:        "http://127.0.0.1/status?format=json",
		Method:     "GET",
		Headers:    make(map[string]string),
		HostHeader: "",
		Timeout:    config.Duration(time.Second * 5),
	}
}

func init() {
	inputs.Add("nginx_upstream_check", func() telegraf.Input {
		return NewNginxUpstreamCheck()
	})
}

func (check *NginxUpstreamCheck) SampleConfig() string {
	return sampleConfig
}

func (check *NginxUpstreamCheck) Description() string {
	return description
}

type NginxUpstreamCheckData struct {
	Servers struct {
		Total      uint64                     `json:"total"`
		Generation uint64                     `json:"generation"`
		Server     []NginxUpstreamCheckServer `json:"server"`
	} `json:"servers"`
}

type NginxUpstreamCheckServer struct {
	Index    uint64 `json:"index"`
	Upstream string `json:"upstream"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Rise     uint64 `json:"rise"`
	Fall     uint64 `json:"fall"`
	Type     string `json:"type"`
	Port     uint16 `json:"port"`
}

// createHTTPClient create a clients to access API
func (check *NginxUpstreamCheck) createHTTPClient() (*http.Client, error) {
	tlsConfig, err := check.ClientConfig.TLSConfig()
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: time.Duration(check.Timeout),
	}

	return client, nil
}

// gatherJSONData query the data source and parse the response JSON
func (check *NginxUpstreamCheck) gatherJSONData(address string, value interface{}) error {
	var method string
	if check.Method != "" {
		method = check.Method
	} else {
		method = "GET"
	}

	request, err := http.NewRequest(method, address, nil)
	if err != nil {
		return err
	}

	if (check.Username != "") || (check.Password != "") {
		request.SetBasicAuth(check.Username, check.Password)
	}
	for header, value := range check.Headers {
		request.Header.Add(header, value)
	}
	if check.HostHeader != "" {
		request.Host = check.HostHeader
	}

	response, err := check.client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		// ignore the err here; LimitReader returns io.EOF and we're not interested in read errors.
		body, _ := io.ReadAll(io.LimitReader(response.Body, 200))
		return fmt.Errorf("%s returned HTTP status %s: %q", address, response.Status, body)
	}

	err = json.NewDecoder(response.Body).Decode(value)
	if err != nil {
		return err
	}

	return nil
}

func (check *NginxUpstreamCheck) Gather(accumulator telegraf.Accumulator) error {
	if check.client == nil {
		client, err := check.createHTTPClient()

		if err != nil {
			return err
		}
		check.client = client
	}

	statusURL, err := url.Parse(check.URL)
	if err != nil {
		return err
	}

	err = check.gatherStatusData(statusURL.String(), accumulator)
	if err != nil {
		return err
	}

	return nil
}

func (check *NginxUpstreamCheck) gatherStatusData(address string, accumulator telegraf.Accumulator) error {
	checkData := &NginxUpstreamCheckData{}

	err := check.gatherJSONData(address, checkData)
	if err != nil {
		return err
	}

	for _, server := range checkData.Servers.Server {
		tags := map[string]string{
			"upstream": server.Upstream,
			"type":     server.Type,
			"name":     server.Name,
			"port":     strconv.Itoa(int(server.Port)),
			"url":      address,
		}

		fields := map[string]interface{}{
			"status":      server.Status,
			"status_code": check.getStatusCode(server.Status),
			"rise":        server.Rise,
			"fall":        server.Fall,
		}

		accumulator.AddFields("nginx_upstream_check", fields, tags)
	}

	return nil
}

func (check *NginxUpstreamCheck) getStatusCode(status string) uint8 {
	switch status {
	case "up":
		return 1
	case "down":
		return 2
	default:
		return 0
	}
}
