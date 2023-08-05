package main

import (
	"fmt"

	"github.com/pulumi/pulumi-eks/sdk/go/eks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		config := config.New(ctx, "")
		shouldDeploy := config.RequireBool("deploy")
		if shouldDeploy {
			createEksCluster(ctx, config)
		}

		// ctx.Export("clusterName", cluster.Name)
		// ctx.Export("vpcId", vpcId)
		// ctx.Export("privateSubnetIds", privateSubnetIdsOutput)

		return nil
	})
}

func createEksCluster(ctx *pulumi.Context, conifg *config.Config) (*eks.Cluster, error) {
	env := config.Require(ctx, "env")
	k8sVersion := config.Require(ctx, "k8sVersion")

	vpcStackRef, err := pulumi.NewStackReference(ctx, fmt.Sprintf("organization/infrastructure-vpc/%s", env), nil)
	if err != nil {
		return &eks.Cluster{}, err
	}

	vpcId := vpcStackRef.GetStringOutput(pulumi.String("vpcId"))

	privateSubnetIds := vpcStackRef.GetOutput(pulumi.String("privateSubnetIds")).AsStringArrayOutput().ApplyT(func(privateSubnetIds []string) []string {
		return privateSubnetIds
	}).(pulumi.StringArrayOutput)

	cluster, _ := eks.NewCluster(ctx, env, &eks.ClusterArgs{
		CreateOidcProvider:    pulumi.Bool(true),
		EndpointPrivateAccess: pulumi.Bool(true),
		EndpointPublicAccess:  pulumi.Bool(false),
		PublicSubnetIds:       privateSubnetIds,
		// RoleMappings: [],
		SkipDefaultNodeGroup: pulumi.BoolRef(true),
		Tags: pulumi.StringMap{
			"Env":  pulumi.String(env),
			"Name": pulumi.String(env),
		},
		Version: pulumi.String(k8sVersion),
		VpcId:   vpcId,
	})
	return cluster, nil
}
