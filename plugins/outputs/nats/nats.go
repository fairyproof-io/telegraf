package nats

import (
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"

	"github.com/fairyproof-io/telegraf"
	"github.com/fairyproof-io/telegraf/plugins/common/tls"
	"github.com/fairyproof-io/telegraf/plugins/outputs"
	"github.com/fairyproof-io/telegraf/plugins/serializers"
)

type NATS struct {
	Servers     []string `toml:"servers"`
	Secure      bool     `toml:"secure"`
	Name        string   `toml:"name"`
	Username    string   `toml:"username"`
	Password    string   `toml:"password"`
	Credentials string   `toml:"credentials"`
	Subject     string   `toml:"subject"`

	tls.ClientConfig

	Log telegraf.Logger `toml:"-"`

	conn       *nats.Conn
	serializer serializers.Serializer
}

var sampleConfig = `
  ## URLs of NATS servers
  servers = ["nats://localhost:4222"]

  ## Optional client name
  # name = ""

  ## Optional credentials
  # username = ""
  # password = ""

  ## Optional NATS 2.0 and NATS NGS compatible user credentials
  # credentials = "/etc/telegraf/nats.creds"

  ## NATS subject for producer messages
  subject = "telegraf"

  ## Use Transport Layer Security
  # secure = false

  ## Optional TLS Config
  # tls_ca = "/etc/telegraf/ca.pem"
  # tls_cert = "/etc/telegraf/cert.pem"
  # tls_key = "/etc/telegraf/key.pem"
  ## Use TLS but skip chain & host verification
  # insecure_skip_verify = false

  ## Data format to output.
  ## Each data format has its own unique set of configuration options, read
  ## more about them here:
  ## https://github.com/fairyproof-io/telegraf/blob/master/docs/DATA_FORMATS_OUTPUT.md
  data_format = "influx"
`

func (n *NATS) SetSerializer(serializer serializers.Serializer) {
	n.serializer = serializer
}

func (n *NATS) Connect() error {
	var err error

	opts := []nats.Option{
		nats.MaxReconnects(-1),
	}

	// override authentication, if any was specified
	if n.Username != "" && n.Password != "" {
		opts = append(opts, nats.UserInfo(n.Username, n.Password))
	}

	if n.Credentials != "" {
		opts = append(opts, nats.UserCredentials(n.Credentials))
	}

	if n.Name != "" {
		opts = append(opts, nats.Name(n.Name))
	}

	if n.Secure {
		tlsConfig, err := n.ClientConfig.TLSConfig()
		if err != nil {
			return err
		}

		opts = append(opts, nats.Secure(tlsConfig))
	}

	// try and connect
	n.conn, err = nats.Connect(strings.Join(n.Servers, ","), opts...)

	return err
}

func (n *NATS) Close() error {
	n.conn.Close()
	return nil
}

func (n *NATS) SampleConfig() string {
	return sampleConfig
}

func (n *NATS) Description() string {
	return "Send telegraf measurements to NATS"
}

func (n *NATS) Write(metrics []telegraf.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	for _, metric := range metrics {
		buf, err := n.serializer.Serialize(metric)
		if err != nil {
			n.Log.Debugf("Could not serialize metric: %v", err)
			continue
		}

		err = n.conn.Publish(n.Subject, buf)
		if err != nil {
			return fmt.Errorf("FAILED to send NATS message: %s", err)
		}
	}
	return nil
}

func init() {
	outputs.Add("nats", func() telegraf.Output {
		return &NATS{}
	})
}
