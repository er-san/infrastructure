package main

import (
	"encoding/json"

	"github.com/pulumi/pulumi-awsx/sdk/go/awsx/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type CreateVpcResult struct {
	azs interface{}
	vpc *ec2.Vpc
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		config := config.New(ctx, "")
		result, _ := createVpc(ctx, config)

		ctx.Export("availabilityZones", pulumi.ToOutput(result.azs))
		ctx.Export("privateSubnetIds", result.vpc.PrivateSubnetIds)
		ctx.Export("publicSubnetIds", result.vpc.PublicSubnetIds)
		ctx.Export("vpcId", result.vpc.VpcId)
		ctx.Export("vpcCidrBlock", result.vpc.Vpc.CidrBlock())
		return nil
	})
}

func createVpc(ctx *pulumi.Context, conifg *config.Config) (CreateVpcResult, error) {
	azsValue := config.Require(ctx, "availabilityZoneNames")

	var azs []string
	if err := json.Unmarshal([]byte(azsValue), &azs); err != nil {
		return CreateVpcResult{}, err
	}

	cidr := config.Require(ctx, "cidr")
	env := config.Require(ctx, "env")
	vpc, err := ec2.NewVpc(ctx, env, &ec2.VpcArgs{
		CidrBlock:             &cidr,
		AvailabilityZoneNames: azs,
		Tags: pulumi.StringMap{
			"Name": pulumi.String(env),
			"Env":  pulumi.String(env),
		},
	})

	if err != nil {
		return CreateVpcResult{}, err
	}

	return CreateVpcResult{
		vpc: vpc,
		azs: azs,
	}, err
}
