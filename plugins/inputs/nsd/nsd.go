package nsd

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/fairyproof-io/telegraf"
	"github.com/fairyproof-io/telegraf/config"
	"github.com/fairyproof-io/telegraf/internal"
	"github.com/fairyproof-io/telegraf/plugins/inputs"
)

type runner func(cmdName string, timeout config.Duration, useSudo bool, Server string, ConfigFile string) (*bytes.Buffer, error)

// NSD is used to store configuration values
type NSD struct {
	Binary     string
	Timeout    config.Duration
	UseSudo    bool
	Server     string
	ConfigFile string

	run runner
}

var defaultBinary = "/usr/sbin/nsd-control"
var defaultTimeout = config.Duration(time.Second)

var sampleConfig = `
  ## Address of server to connect to, optionally ':port'. Defaults to the
  ## address in the nsd config file.
  server = "127.0.0.1:8953"

  ## If running as a restricted user you can prepend sudo for additional access:
  # use_sudo = false

  ## The default location of the nsd-control binary can be overridden with:
  # binary = "/usr/sbin/nsd-control"

  ## The default location of the nsd config file can be overridden with:
  # config_file = "/etc/nsd/nsd.conf"

  ## The default timeout of 1s can be overridden with:
  # timeout = "1s"
`

// Description displays what this plugin is about
func (s *NSD) Description() string {
	return "A plugin to collect stats from the NSD authoritative DNS name server"
}

// SampleConfig displays configuration instructions
func (s *NSD) SampleConfig() string {
	return sampleConfig
}

// Shell out to nsd_stat and return the output
func nsdRunner(cmdName string, timeout config.Duration, useSudo bool, server string, configFile string) (*bytes.Buffer, error) {
	cmdArgs := []string{"stats_noreset"}

	if server != "" {
		host, port, err := net.SplitHostPort(server)
		if err == nil {
			server = host + "@" + port
		}

		cmdArgs = append([]string{"-s", server}, cmdArgs...)
	}

	if configFile != "" {
		cmdArgs = append([]string{"-c", configFile}, cmdArgs...)
	}

	cmd := exec.Command(cmdName, cmdArgs...)

	if useSudo {
		cmdArgs = append([]string{cmdName}, cmdArgs...)
		cmd = exec.Command("sudo", cmdArgs...)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	err := internal.RunTimeout(cmd, time.Duration(timeout))
	if err != nil {
		return &out, fmt.Errorf("error running nsd-control: %s (%s %v)", err, cmdName, cmdArgs)
	}

	return &out, nil
}

// Gather collects stats from nsd-control and adds them to the Accumulator
func (s *NSD) Gather(acc telegraf.Accumulator) error {
	out, err := s.run(s.Binary, s.Timeout, s.UseSudo, s.Server, s.ConfigFile)
	if err != nil {
		return fmt.Errorf("error gathering metrics: %s", err)
	}

	// Process values
	fields := make(map[string]interface{})
	fieldsServers := make(map[string]map[string]interface{})

	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		cols := strings.Split(scanner.Text(), "=")

		// Check split correctness
		if len(cols) != 2 {
			continue
		}

		stat := cols[0]
		value := cols[1]

		fieldValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			acc.AddError(fmt.Errorf("expected a numerical value for %s = %v",
				stat, value))
			continue
		}

		if strings.HasPrefix(stat, "server") {
			statTokens := strings.Split(stat, ".")
			if len(statTokens) > 1 {
				serverID := strings.TrimPrefix(statTokens[0], "server")
				if _, err := strconv.Atoi(serverID); err == nil {
					serverTokens := statTokens[1:]
					field := strings.Join(serverTokens[:], "_")
					if fieldsServers[serverID] == nil {
						fieldsServers[serverID] = make(map[string]interface{})
					}
					fieldsServers[serverID][field] = fieldValue
				}
			}
		} else {
			field := strings.Replace(stat, ".", "_", -1)
			fields[field] = fieldValue
		}
	}

	acc.AddFields("nsd", fields, nil)
	for thisServerID, thisServerFields := range fieldsServers {
		thisServerTag := map[string]string{"server": thisServerID}
		acc.AddFields("nsd_servers", thisServerFields, thisServerTag)
	}

	return nil
}

func init() {
	inputs.Add("nsd", func() telegraf.Input {
		return &NSD{
			run:        nsdRunner,
			Binary:     defaultBinary,
			Timeout:    defaultTimeout,
			UseSudo:    false,
			Server:     "",
			ConfigFile: "",
		}
	})
}
