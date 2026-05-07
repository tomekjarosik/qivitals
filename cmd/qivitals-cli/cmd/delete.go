package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
)

func NewCmdDelete() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "delete [flags]",
		Short: "Delete a sensor and all its history",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, conn, err := NewStatusClient(cmd.Context())
			if err != nil {
				return err
			}
			defer conn.Close()

			_, err = client.DeleteSensor(cmd.Context(), &v1.DeleteSensorRequest{Id: id})
			if err != nil {
				return fmt.Errorf("failed to delete sensor: %w", err)
			}
			fmt.Printf("Sensor '%s' deleted successfully.\n", id)

			return nil
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Unique sensor UUID to delete (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}
