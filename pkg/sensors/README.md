# qivitals-sensors

A highly extensible, plugin-like Go package for collecting system telemetry and hardware status.

`qivitals-sensors` provides a unified interface for interacting with various system components (ZFS, disks, compute resources, and more). It is designed to be the data-collection engine for the [QiVitals](https://github.com/tomekjarosik/qivitals) ecosystem, where all data is normalized into simple key-value pairs.

## Features

- **Universal Data Format**: Every sensor reports data as `map[string]string`, making it trivial to consume in CLI tools, web services, or monitoring backends.
- **Plugin-like Architecture**: The `Registry` system allows sensors to be discovered and instantiated dynamically by name.
- **Context-Aware**: Full support for `context.Context` for timeouts, graceful cancellation, and signal handling.
- **Community Driven**: Built for contributions. If you have a way to query a specific hardware or software stack, you can easily plug it in.

## Architecture

The package revolves around two main concepts:

### 1. The `SensorReader` Interface
Every sensor must implement this interface. It standardizes how a sensor identifies itself, executes its logic, and returns results.

```textmate
type SensorReader interface {
    // Kind returns a unique identifier for the sensor type (e.g., "zpool", "disk")
    Kind() string

    // Execute runs the sensor logic and populates the results.
    // 'args' can be used to pass specific parameters (e.g., a pool name or device path).
    Execute(ctx context.Context, args []string) error

    // Results returns the collected metrics as key-value pairs.
    Results() map[string]string
}
```


### 2. The `Registry`
The `Registry` manages all available sensor types. Sensors register themselves (typically in an `init()` function), and consumers can look them up by name without needing to import their specific implementation packages.

## Usage

### Using a sensor programmatically

```textmate
package main

import (
    "context"
    "fmt"
    
    "github.com/tomekjarosik/qivitals/pkg/sensors"
    // Import the specific sensor to trigger its init() registration
    _ "github.com/tomekjarosik/qivitals/pkg/sensors/zpool"
)

func main() {
    // Retrieve the global registry
    reg := sensors.DefaultRegistry()

    // Create a new sensor by type name. "tank" is passed as an argument.
    reader, err := reg.Create("zpool", "tank")
    if err != nil {
        panic(err)
    }

    // Execute the sensor
    ctx := context.Background()
    if err := reader.Execute(ctx, []string{"tank"}); err != nil {
        panic(err)
    }

    // Access results
    for k, v := range reader.Results() {
        fmt.Printf("%s: %s\n", k, v)
    }
}
```

## Contributing

We want this package to become the go-to library for system telemetry! Because of the registry architecture, adding a new sensor is incredibly simple.

### Step 1: Create your sensor file
Create a new Go file in this package (e.g., `mysensor.go`).

### Step 2: Implement `SensorReader`
Your struct needs to store the configuration and the results.

```textmate
type MySensor struct {
    target  string
    results map[string]string
}

func NewMySensor(target string) *MySensor {
    return &MySensor{
        target:  target,
        results: make(map[string]string),
    }
}

func (s *MySensor) Kind() string {
    return "mysensor"
}

func (s *MySensor) Execute(ctx context.Context, args []string) error {
    // 1. Perform your logic here (exec.Command, API calls, file reads, etc.)
    // 2. Store metrics in s.results
    s.results["status"] = "healthy"
    s.results["uptime"] = "42s"
    return nil
}

func (s *MySensor) Results() map[string]string {
    return s.results
}
```


### Step 3: Register the sensor
Add an `init()` function to automatically register your sensor with the global registry when the package is imported.

```textmate
func init() {
    sensors.RegisterDefault("mysensor", func(args ...string) (sensors.SensorReader, error) {
        if len(args) == 0 {
            return nil, fmt.Errorf("target name is required for mysensor")
        }
        return NewMySensor(args[0]), nil
    })
}
```


### Step 4: Submit a Pull Request
Ensure your sensor handles errors gracefully, respects `context.Context` for cancellations, and includes a brief description in the **Available Sensors** table above.

## Notes on Error Handling

There is an important distinction between execution errors and hardware status errors:

1. **Execution Errors**: Return an `error` in `Execute` if the sensor cannot run (e.g., missing system tools, permission denied, invalid arguments).
2. **Hardware Status**: If the sensor runs successfully but the hardware is failing, return `nil` and store the failure in the results map (e.g., `results["status"] = "critical"`).

This allows consumers to distinguish between *"I couldn't read the sensor"* and *"The sensor successfully read that things are broken."*

