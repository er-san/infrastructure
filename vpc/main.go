package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		other, err := pulumi.NewStackReference(ctx, "organization/infrastructure-k8s/dev", nil)
		if err != nil {
			return err
		}
		name := other.GetOutput(pulumi.String("name"))
		fmt.Sprintln(name)
		return nil
	})
}
