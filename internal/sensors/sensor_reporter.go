package sensors

import "context"

type SensorReader interface {
	Kind() string
	Execute(ctx context.Context, args []string) error
	Results() map[string]string
}
