package sensors

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func init() {
	RegisterDefault("zpool", func(args ...string) (SensorReader, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("pool name must be provided")
		}
		poolName := args[0]
		return NewZpoolSensor(poolName), nil
	})
}

// zpoolJSON represents the top-level structure of `zpool status -j` output.
type zpoolJSON struct {
	Pools map[string]*poolData `json:"pools"`
}

// poolData contains information about a single ZFS pool.
type poolData struct {
	Name       string               `json:"name"`
	State      string               `json:"state"`
	Status     string               `json:"status"`
	Action     string               `json:"action"`
	ErrorCount int64                `json:"error_count"`
	ScanStats  *scanData            `json:"scan_stats"`
	Vdevs      map[string]*vdevData `json:"vdevs"`
}

// scanData contains scrub/scan statistics.
type scanData struct {
	Function           string `json:"function"`
	State              string `json:"state"`
	StartTime          int64  `json:"start_time"`
	EndTime            int64  `json:"end_time"`
	ToExamine          int64  `json:"to_examine"`
	Examined           int64  `json:"examined"`
	Skipped            int64  `json:"skipped"`
	Processed          int64  `json:"processed"`
	Errors             int64  `json:"errors"`
	BytesPerScan       int64  `json:"bytes_per_scan"`
	PassStart          int64  `json:"pass_start"`
	ScrubPause         int64  `json:"scrub_pause"`
	ScrubSpentPaused   int64  `json:"scrub_spent_paused"`
	IssuedBytesPerScan int64  `json:"issued_bytes_per_scan"`
	Issued             int64  `json:"issued"`
}

// vdevData represents a VDEV's information.
type vdevData struct {
	Name           string               `json:"name"`
	VdevType       string               `json:"vdev_type"`
	State          string               `json:"state"`
	AllocSpace     int64                `json:"alloc_space"`
	TotalSpace     int64                `json:"total_space"`
	ReadErrors     int64                `json:"read_errors"`
	WriteErrors    int64                `json:"write_errors"`
	ChecksumErrors int64                `json:"checksum_errors"`
	Vdevs          map[string]*vdevData `json:"vdevs"`
}

// ZpoolSensor implements SensorReader for ZFS zpool status.
type ZpoolSensor struct {
	poolName string
	results  map[string]string
}

// NewZpoolSensor creates a new ZFS zpool sensor for the given pool name.
func NewZpoolSensor(poolName string) *ZpoolSensor {
	return &ZpoolSensor{
		poolName: poolName,
		results:  make(map[string]string),
	}
}

// Kind returns the sensor type identifier.
func (s *ZpoolSensor) Kind() string {
	return "zpool"
}

// Execute runs `zpool status -j --json-int` to fetch exact numeric values.
// If poolName is also provided via args[0], it takes precedence.
func (s *ZpoolSensor) Execute(ctx context.Context, args []string) error {
	poolName := s.poolName
	if len(args) > 0 && args[0] != "" {
		poolName = args[0]
	}

	cmd := exec.CommandContext(ctx, "zpool", "status", "-j", "--json-int", poolName)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("zpool status failed: %w", err)
	}

	var data zpoolJSON
	if err := json.Unmarshal(output, &data); err != nil {
		return fmt.Errorf("parse zpool JSON: %w", err)
	}

	pool, ok := data.Pools[poolName]
	if !ok {
		return fmt.Errorf("pool %q not found", poolName)
	}

	var rootVdev *vdevData
	if vdev, ok := pool.Vdevs[pool.Name]; ok {
		rootVdev = vdev
	}

	results := make(map[string]string)
	results["name"] = pool.Name
	results["state"] = pool.State
	results["health"] = pool.State

	if rootVdev != nil {
		results["used_space"] = strconv.FormatInt(rootVdev.AllocSpace, 10)
		results["total_space"] = strconv.FormatInt(rootVdev.TotalSpace, 10)
		if rootVdev.AllocSpace > 0 || rootVdev.TotalSpace > 0 {
			results["capacity"] = fmt.Sprintf("%s/%s", results["used_space"], results["total_space"])
		}
		results["used_bytes"] = results["used_space"]
		results["total_bytes"] = results["total_space"]
		results["read_errors"] = strconv.FormatInt(rootVdev.ReadErrors, 10)
		results["write_errors"] = strconv.FormatInt(rootVdev.WriteErrors, 10)
		results["checksum_errors"] = strconv.FormatInt(rootVdev.ChecksumErrors, 10)
	}

	results["error_count"] = strconv.FormatInt(pool.ErrorCount, 10)

	if pool.ScanStats != nil {
		results["scrub_function"] = pool.ScanStats.Function
		results["scrub_state"] = pool.ScanStats.State
		results["scrub_start"] = strconv.FormatInt(pool.ScanStats.StartTime, 10)
		results["scrub_end"] = strconv.FormatInt(pool.ScanStats.EndTime, 10)
		results["scrub_errors"] = strconv.FormatInt(pool.ScanStats.Errors, 10)
	}

	if pool.Status != "" {
		results["status"] = strings.TrimSpace(pool.Status)
	}
	if pool.Action != "" {
		results["action"] = strings.TrimSpace(pool.Action)
	}

	s.results = results
	return nil
}

// Results returns the collected metrics.
// Returns an empty map if Execute() has not been called or failed.
func (s *ZpoolSensor) Results() map[string]string {
	return s.results
}
