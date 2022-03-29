//go:generate go run ../../../tools/generate_plugindata/main.go
//go:generate go run ../../../tools/generate_plugindata/main.go --clean
package jenkins

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/filter"
	"github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/influxdata/telegraf/plugins/inputs"
)

// Jenkins plugin gathers information about the nodes and jobs running in a jenkins instance.
type Jenkins struct {
	URL      string
	Username string
	Password string
	Source   string
	Port     string
	// HTTP Timeout specified as a string - 3s, 1m, 1h
	ResponseTimeout config.Duration

	tls.ClientConfig
	client *client

	Log telegraf.Logger

	MaxConnections    int             `toml:"max_connections"`
	MaxBuildAge       config.Duration `toml:"max_build_age"`
	MaxSubJobDepth    int             `toml:"max_subjob_depth"`
	MaxSubJobPerLayer int             `toml:"max_subjob_per_layer"`
	JobExclude        []string        `toml:"job_exclude"`
	JobInclude        []string        `toml:"job_include"`
	jobFilter         filter.Filter

	NodeExclude []string `toml:"node_exclude"`
	NodeInclude []string `toml:"node_include"`
	nodeFilter  filter.Filter

	semaphore chan struct{}
}

// measurement
const (
	measurementJenkins = "jenkins"
	measurementNode    = "jenkins_node"
	measurementJob     = "jenkins_job"
)

// SampleConfig implements telegraf.Input interface
func (j *Jenkins) SampleConfig() string {
	return `{{ .SampleConfig }}`
}

// Gather implements telegraf.Input interface
func (j *Jenkins) Gather(acc telegraf.Accumulator) error {
	if j.client == nil {
		client, err := j.newHTTPClient()
		if err != nil {
			return err
		}
		if err = j.initialize(client); err != nil {
			return err
		}
	}

	j.gatherNodesData(acc)
	j.gatherJobs(acc)

	return nil
}

func (j *Jenkins) newHTTPClient() (*http.Client, error) {
	tlsCfg, err := j.ClientConfig.TLSConfig()
	if err != nil {
		return nil, fmt.Errorf("error parse jenkins config[%s]: %v", j.URL, err)
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
			MaxIdleConns:    j.MaxConnections,
		},
		Timeout: time.Duration(j.ResponseTimeout),
	}, nil
}

// separate the client as dependency to use httptest Client for mocking
func (j *Jenkins) initialize(client *http.Client) error {
	var err error

	// init jenkins tags
	u, err := url.Parse(j.URL)
	if err != nil {
		return err
	}
	if u.Port() == "" {
		if u.Scheme == "http" {
			j.Port = "80"
		} else if u.Scheme == "https" {
			j.Port = "443"
		}
	} else {
		j.Port = u.Port()
	}
	j.Source = u.Hostname()

	// init filters
	j.jobFilter, err = filter.NewIncludeExcludeFilter(j.JobInclude, j.JobExclude)
	if err != nil {
		return fmt.Errorf("error compiling job filters[%s]: %v", j.URL, err)
	}
	j.nodeFilter, err = filter.NewIncludeExcludeFilter(j.NodeInclude, j.NodeExclude)
	if err != nil {
		return fmt.Errorf("error compiling node filters[%s]: %v", j.URL, err)
	}

	// init tcp pool with default value
	if j.MaxConnections <= 0 {
		j.MaxConnections = 5
	}

	// default sub jobs can be acquired
	if j.MaxSubJobPerLayer <= 0 {
		j.MaxSubJobPerLayer = 10
	}

	j.semaphore = make(chan struct{}, j.MaxConnections)

	j.client = newClient(client, j.URL, j.Username, j.Password, j.MaxConnections)

	return j.client.init()
}

func (j *Jenkins) gatherNodeData(n node, acc telegraf.Accumulator) error {
	tags := map[string]string{}
	if n.DisplayName == "" {
		return fmt.Errorf("error empty node name")
	}

	tags["node_name"] = n.DisplayName

	// filter out excluded or not included node_name
	if !j.nodeFilter.Match(tags["node_name"]) {
		return nil
	}

	monitorData := n.MonitorData

	if monitorData.HudsonNodeMonitorsArchitectureMonitor != "" {
		tags["arch"] = monitorData.HudsonNodeMonitorsArchitectureMonitor
	}

	tags["status"] = "online"
	if n.Offline {
		tags["status"] = "offline"
	}

	tags["source"] = j.Source
	tags["port"] = j.Port

	fields := make(map[string]interface{})
	fields["num_executors"] = n.NumExecutors

	if monitorData.HudsonNodeMonitorsResponseTimeMonitor != nil {
		fields["response_time"] = monitorData.HudsonNodeMonitorsResponseTimeMonitor.Average
	}
	if monitorData.HudsonNodeMonitorsDiskSpaceMonitor != nil {
		tags["disk_path"] = monitorData.HudsonNodeMonitorsDiskSpaceMonitor.Path
		fields["disk_available"] = monitorData.HudsonNodeMonitorsDiskSpaceMonitor.Size
	}
	if monitorData.HudsonNodeMonitorsTemporarySpaceMonitor != nil {
		tags["temp_path"] = monitorData.HudsonNodeMonitorsTemporarySpaceMonitor.Path
		fields["temp_available"] = monitorData.HudsonNodeMonitorsTemporarySpaceMonitor.Size
	}
	if monitorData.HudsonNodeMonitorsSwapSpaceMonitor != nil {
		fields["swap_available"] = monitorData.HudsonNodeMonitorsSwapSpaceMonitor.SwapAvailable
		fields["memory_available"] = monitorData.HudsonNodeMonitorsSwapSpaceMonitor.MemoryAvailable
		fields["swap_total"] = monitorData.HudsonNodeMonitorsSwapSpaceMonitor.SwapTotal
		fields["memory_total"] = monitorData.HudsonNodeMonitorsSwapSpaceMonitor.MemoryTotal
	}
	acc.AddFields(measurementNode, fields, tags)

	return nil
}

func (j *Jenkins) gatherNodesData(acc telegraf.Accumulator) {
	nodeResp, err := j.client.getAllNodes(context.Background())
	if err != nil {
		acc.AddError(err)
		return
	}

	// get total and busy executors
	tags := map[string]string{"source": j.Source, "port": j.Port}
	fields := make(map[string]interface{})
	fields["busy_executors"] = nodeResp.BusyExecutors
	fields["total_executors"] = nodeResp.TotalExecutors

	acc.AddFields(measurementJenkins, fields, tags)

	// get node data
	for _, node := range nodeResp.Computers {
		err = j.gatherNodeData(node, acc)
		if err == nil {
			continue
		}
		acc.AddError(err)
	}
}

func (j *Jenkins) gatherJobs(acc telegraf.Accumulator) {
	js, err := j.client.getJobs(context.Background(), nil)
	if err != nil {
		acc.AddError(err)
		return
	}
	var wg sync.WaitGroup
	for _, job := range js.Jobs {
		wg.Add(1)
		go func(name string, wg *sync.WaitGroup, acc telegraf.Accumulator) {
			defer wg.Done()
			if err := j.getJobDetail(jobRequest{
				name:    name,
				parents: []string{},
				layer:   0,
			}, acc); err != nil {
				acc.AddError(err)
			}
		}(job.Name, &wg, acc)
	}
	wg.Wait()
}

func (j *Jenkins) getJobDetail(jr jobRequest, acc telegraf.Accumulator) error {
	if j.MaxSubJobDepth > 0 && jr.layer == j.MaxSubJobDepth {
		return nil
	}

	// filter out excluded or not included jobs
	if !j.jobFilter.Match(jr.hierarchyName()) {
		return nil
	}

	js, err := j.client.getJobs(context.Background(), &jr)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for k, ij := range js.Jobs {
		if k < len(js.Jobs)-j.MaxSubJobPerLayer-1 {
			continue
		}
		wg.Add(1)
		// schedule tcp fetch for inner jobs
		go func(ij innerJob, jr jobRequest, acc telegraf.Accumulator) {
			defer wg.Done()
			if err := j.getJobDetail(jobRequest{
				name:    ij.Name,
				parents: jr.combined(),
				layer:   jr.layer + 1,
			}, acc); err != nil {
				acc.AddError(err)
			}
		}(ij, jr, acc)
	}
	wg.Wait()

	// collect build info
	number := js.LastBuild.Number
	if number < 1 {
		// no build info
		return nil
	}
	build, err := j.client.getBuild(context.Background(), jr, number)
	if err != nil {
		return err
	}

	if build.Building {
		j.Log.Debugf("Ignore running build on %s, build %v", jr.name, number)
		return nil
	}

	// stop if build is too old
	// Higher up in gatherJobs
	cutoff := time.Now().Add(-1 * time.Duration(j.MaxBuildAge))

	// Here we just test
	if build.GetTimestamp().Before(cutoff) {
		return nil
	}

	j.gatherJobBuild(jr, build, acc)
	return nil
}

type nodeResponse struct {
	Computers      []node `json:"computer"`
	BusyExecutors  int    `json:"busyExecutors"`
	TotalExecutors int    `json:"totalExecutors"`
}

type node struct {
	DisplayName  string      `json:"displayName"`
	Offline      bool        `json:"offline"`
	NumExecutors int         `json:"numExecutors"`
	MonitorData  monitorData `json:"monitorData"`
}

type monitorData struct {
	HudsonNodeMonitorsArchitectureMonitor   string               `json:"hudson.node_monitors.ArchitectureMonitor"`
	HudsonNodeMonitorsDiskSpaceMonitor      *nodeSpaceMonitor    `json:"hudson.node_monitors.DiskSpaceMonitor"`
	HudsonNodeMonitorsResponseTimeMonitor   *responseTimeMonitor `json:"hudson.node_monitors.ResponseTimeMonitor"`
	HudsonNodeMonitorsSwapSpaceMonitor      *swapSpaceMonitor    `json:"hudson.node_monitors.SwapSpaceMonitor"`
	HudsonNodeMonitorsTemporarySpaceMonitor *nodeSpaceMonitor    `json:"hudson.node_monitors.TemporarySpaceMonitor"`
}

type nodeSpaceMonitor struct {
	Path string  `json:"path"`
	Size float64 `json:"size"`
}

type responseTimeMonitor struct {
	Average int64 `json:"average"`
}

type swapSpaceMonitor struct {
	SwapAvailable   float64 `json:"availableSwapSpace"`
	SwapTotal       float64 `json:"totalSwapSpace"`
	MemoryAvailable float64 `json:"availablePhysicalMemory"`
	MemoryTotal     float64 `json:"totalPhysicalMemory"`
}

type jobResponse struct {
	LastBuild jobBuild   `json:"lastBuild"`
	Jobs      []innerJob `json:"jobs"`
	Name      string     `json:"name"`
}

type innerJob struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Color string `json:"color"`
}

type jobBuild struct {
	Number int64
	URL    string
}

type buildResponse struct {
	Building  bool   `json:"building"`
	Duration  int64  `json:"duration"`
	Number    int64  `json:"number"`
	Result    string `json:"result"`
	Timestamp int64  `json:"timestamp"`
}

func (b *buildResponse) GetTimestamp() time.Time {
	return time.Unix(0, b.Timestamp*int64(time.Millisecond))
}

const (
	nodePath = "/computer/api/json"
	jobPath  = "/api/json"
)

type jobRequest struct {
	name    string
	parents []string
	layer   int
}

func (jr jobRequest) combined() []string {
	path := make([]string, len(jr.parents))
	copy(path, jr.parents)
	return append(path, jr.name)
}

func (jr jobRequest) combinedEscaped() []string {
	jobs := jr.combined()
	for index, job := range jobs {
		jobs[index] = url.PathEscape(job)
	}
	return jobs
}

func (jr jobRequest) URL() string {
	return "/job/" + strings.Join(jr.combinedEscaped(), "/job/") + jobPath
}

func (jr jobRequest) buildURL(number int64) string {
	return "/job/" + strings.Join(jr.combinedEscaped(), "/job/") + "/" + strconv.Itoa(int(number)) + jobPath
}

func (jr jobRequest) hierarchyName() string {
	return strings.Join(jr.combined(), "/")
}

func (jr jobRequest) parentsString() string {
	return strings.Join(jr.parents, "/")
}

func (j *Jenkins) gatherJobBuild(jr jobRequest, b *buildResponse, acc telegraf.Accumulator) {
	tags := map[string]string{"name": jr.name, "parents": jr.parentsString(), "result": b.Result, "source": j.Source, "port": j.Port}
	fields := make(map[string]interface{})
	fields["duration"] = b.Duration
	fields["result_code"] = mapResultCode(b.Result)
	fields["number"] = b.Number

	acc.AddFields(measurementJob, fields, tags, b.GetTimestamp())
}

// perform status mapping
func mapResultCode(s string) int {
	switch strings.ToLower(s) {
	case "success":
		return 0
	case "failure":
		return 1
	case "not_built":
		return 2
	case "unstable":
		return 3
	case "aborted":
		return 4
	}
	return -1
}

func init() {
	inputs.Add("jenkins", func() telegraf.Input {
		return &Jenkins{
			MaxBuildAge:       config.Duration(time.Hour),
			MaxConnections:    5,
			MaxSubJobPerLayer: 10,
		}
	})
}
