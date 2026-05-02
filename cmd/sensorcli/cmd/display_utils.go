package cmd

import (
	"fmt"

	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func emitJsonFromMessage(m proto.Message) bool {
	if viper.GetBool("machine") {
		// Output raw JSON for machines/E2E tests
		marshaller := protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true}
		jsonStr, err := marshaller.Marshal(m)
		if err != nil {
			return false
		}
		fmt.Println(string(jsonStr))
		return true
	}
	return false
}
