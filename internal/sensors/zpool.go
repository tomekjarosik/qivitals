package sensors

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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
	ErrorCount string               `json:"error_count"`
	ScanStats  *scanData            `json:"scan_stats"`
	Vdevs      map[string]*vdevData `json:"vdevs"`
}

// scanData contains scrub/scan statistics.
type scanData struct {
	Function  string `json:"function"`
	State     string `json:"state"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Errors    string `json:"errors"`
}

// vdevData represents a VDEV's information.
type vdevData struct {
	Name           string               `json:"name"`
	VdevType       string               `json:"vdev_type"`
	State          string               `json:"state"`
	AllocSpace     string               `json:"alloc_space"`
	TotalSpace     string               `json:"total_space"`
	ReadErrors     string               `json:"read_errors"`
	WriteErrors    string               `json:"write_errors"`
	ChecksumErrors string               `json:"checksum_errors"`
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

// Execute runs `zpool status -j <poolname>` and parses the JSON output.
// If poolName is also provided via args[0], it takes precedence.
func (s *ZpoolSensor) Execute(ctx context.Context, args []string) error {
	poolName := s.poolName
	if len(args) > 0 && args[0] != "" {
		poolName = args[0]
	}

	// Run zpool status -j
	cmd := exec.CommandContext(ctx, "zpool", "status", "-j", poolName)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("zpool status failed: %w", err)
	}

	// Parse JSON
	var data zpoolJSON
	if err := json.Unmarshal(output, &data); err != nil {
		return fmt.Errorf("parse zpool JSON: %w", err)
	}

	// Find the requested pool
	pool, ok := data.Pools[poolName]
	if !ok {
		return fmt.Errorf("pool %q not found", poolName)
	}

	// Get root vdev (keyed by pool name in the vdevs map)
	var rootVdev *vdevData
	if vdev, ok := pool.Vdevs[pool.Name]; ok {
		rootVdev = vdev
	}

	// Build results map
	results := make(map[string]string)
	results["name"] = pool.Name
	results["state"] = pool.State
	results["health"] = pool.State

	if rootVdev != nil {
		results["used_space"] = rootVdev.AllocSpace
		results["total_space"] = rootVdev.TotalSpace
		if rootVdev.AllocSpace != "" && rootVdev.TotalSpace != "" {
			results["capacity"] = fmt.Sprintf("%s/%s", rootVdev.AllocSpace, rootVdev.TotalSpace)
		}
		results["read_errors"] = rootVdev.ReadErrors
		results["write_errors"] = rootVdev.WriteErrors
		results["checksum_errors"] = rootVdev.ChecksumErrors
	}

	results["error_count"] = pool.ErrorCount

	if pool.ScanStats != nil {
		results["scrub_function"] = pool.ScanStats.Function
		results["scrub_state"] = pool.ScanStats.State
		results["scrub_start"] = pool.ScanStats.StartTime
		results["scrub_end"] = pool.ScanStats.EndTime
		results["scrub_errors"] = pool.ScanStats.Errors
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
