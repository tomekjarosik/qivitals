package sensors

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

func init() {
	RegisterDefault("smart", func(args ...string) (SensorReader, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("device path (e.g., /dev/sda or /dev/nvme0n1) must be provided")
		}
		return &SmartSensor{device: args[0]}, nil
	})
}

// SmartSensor implements SensorReader for disk health (NVMe & SATA)
type SmartSensor struct {
	device  string
	results map[string]string
}

// smartJSON maps the output of `smartctl -a -j`
type smartJSON struct {
	SmartStatus struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	Temperature struct {
		Current int `json:"current"`
	} `json:"temperature"`
	// NVMe specific health data
	NVMeHealth struct {
		CriticalWarning  int   `json:"critical_warning"`
		AvailableSpare   int   `json:"available_spare"`
		PercentageUsed   int   `json:"percentage_used"`
		DataUnitsRead    int64 `json:"data_units_read"`
		DataUnitsWritten int64 `json:"data_units_written"`
		PowerCycles      int64 `json:"power_cycles"`
		PowerOnHours     int64 `json:"power_on_hours"`
		UnsafeShutdowns  int64 `json:"unsafe_shutdowns"`
		MediaErrors      int64 `json:"media_errors"`
		NumErrLogEntries int64 `json:"num_err_log_entries"`
	} `json:"nvme_smart_health_information_log"`
	// SATA/ATA specific attributes
	ATAAttributes struct {
		Table []struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	UserCapacity struct {
		Bytes int64 `json:"bytes"`
	} `json:"user_capacity"`
	Device struct {
		Type string `json:"type"`
	} `json:"device"`
	ModelName       string `json:"model_name"`
	SerialNumber    string `json:"serial_number"`
	FirmwareVersion string `json:"firmware_version"`
}

func (s *SmartSensor) Kind() string {
	return "smart"
}

func (s *SmartSensor) Execute(ctx context.Context, args []string) error {
	// smartctl -a -j outputs comprehensive JSON for all drive types
	cmd := exec.CommandContext(ctx, "smartctl", "-a", "-j", s.device)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("smartctl failed: %w", err)
	}

	var data smartJSON
	if err := json.Unmarshal(output, &data); err != nil {
		return fmt.Errorf("parse smartctl JSON: %w", err)
	}

	s.results = make(map[string]string)

	// Common metadata
	s.results["device"] = s.device
	s.results["protocol"] = data.Device.Type
	s.results["model"] = data.ModelName
	s.results["serial"] = data.SerialNumber
	s.results["firmware"] = data.FirmwareVersion
	s.results["capacity_bytes"] = strconv.FormatInt(data.UserCapacity.Bytes, 10)
	s.results["health_status"] = strconv.FormatBool(data.SmartStatus.Passed)
	s.results["temperature_celsius"] = strconv.Itoa(data.Temperature.Current)

	// NVMe Specific Metrics
	if data.Device.Type == "nvme" {
		s.results["health_percentage_used"] = strconv.Itoa(data.NVMeHealth.PercentageUsed)
		s.results["available_spare_percent"] = strconv.Itoa(data.NVMeHealth.AvailableSpare)
		s.results["power_on_hours"] = strconv.FormatInt(data.NVMeHealth.PowerOnHours, 10)
		s.results["unsafe_shutdowns"] = strconv.FormatInt(data.NVMeHealth.UnsafeShutdowns, 10)
		s.results["media_errors"] = strconv.FormatInt(data.NVMeHealth.MediaErrors, 10)
		s.results["critical_warning"] = strconv.Itoa(data.NVMeHealth.CriticalWarning)

		// NVMe data units are in 512-byte blocks. Converting to exact bytes.
		s.results["data_read_bytes"] = strconv.FormatInt(data.NVMeHealth.DataUnitsRead*512, 10)
		s.results["data_written_bytes"] = strconv.FormatInt(data.NVMeHealth.DataUnitsWritten*512, 10)
	}

	// ATA/SATA Fallback (common for R730 mechanical drives)
	if data.Device.Type == "ata" || data.Device.Type == "scsi" {
		for _, attr := range data.ATAAttributes.Table {
			switch attr.Name {
			case "Reallocated_Sector_Ct":
				s.results["reallocated_sectors"] = strconv.Itoa(attr.Value)
			case "Current_Pending_Sector":
				s.results["pending_sectors"] = strconv.Itoa(attr.Value)
			case "Offline_Uncorrectable":
				s.results["uncorrectable_sectors"] = strconv.Itoa(attr.Value)
			case "Power_On_Hours":
				s.results["power_on_hours"] = strconv.Itoa(attr.Value)
			case "Wear_Leveling_Count":
				s.results["wear_leveling_count"] = strconv.Itoa(attr.Value)
			}
		}
	}

	return nil
}

func (s *SmartSensor) Results() map[string]string {
	return s.results
}
