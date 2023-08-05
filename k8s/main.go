package main

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi-eks/sdk/go/eks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type CreateEksClusterInput struct {
	ctx                    *pulumi.Context
	eksClusterIamRole      *iam.Role
	enabledClusterLogTypes []string
	env                    string
	k8sVersion             string
	publicSubnetIds        pulumi.StringArrayOutput
	vpcId                  pulumi.StringOutput
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		config := config.New(ctx, "")
		env := config.Require("env")
		k8sVersion := config.Require("k8sVersion")
		enabledClusterLogTypesValues := config.Require("enabledClusterLogTypes")

		var enabledClusterLogTypes []string
		if err := json.Unmarshal([]byte(enabledClusterLogTypesValues), &enabledClusterLogTypes); err != nil {
			return err
		}

		vpcStackRef, err := pulumi.NewStackReference(ctx, fmt.Sprintf("organization/infrastructure-vpc/%s", env), nil)
		if err != nil {
			return err
		}

		vpcId := vpcStackRef.GetStringOutput(pulumi.String("vpcId"))

		publicSubnetIds := vpcStackRef.GetOutput(pulumi.String("publicSubnetIds")).AsStringArrayOutput().ApplyT(func(publicSubnetIds []string) []string {
			return publicSubnetIds
		}).(pulumi.StringArrayOutput)

		eksClusterIamRole, _ := creteEksClusterIamRole(ctx, env)
		shouldDeploy := config.RequireBool("deploy")
		var cluster *eks.Cluster
		if shouldDeploy {
			cluster, _ = createEksCluster(CreateEksClusterInput{
				ctx,
				eksClusterIamRole,
				enabledClusterLogTypes,
				env,
				k8sVersion,
				publicSubnetIds,
				vpcId,
			})
		}

		ctx.Export("clusterName", cluster.EksCluster.Name())
		ctx.Export("vpcId", vpcId.ApplyT(func(vpcId string) string { return vpcId }))
		ctx.Export("publicSubnetIds", publicSubnetIds)

		return nil
	})
}

func createEksCluster(args CreateEksClusterInput) (*eks.Cluster, error) {
	cluster, _ := eks.NewCluster(args.ctx, args.env, &eks.ClusterArgs{
		CreateOidcProvider: pulumi.Bool(true),
		// EnabledClusterLogTypes: ,
		EndpointPrivateAccess: pulumi.Bool(true),
		EndpointPublicAccess:  pulumi.Bool(false),
		InstanceRoles: iam.RoleArray{
			args.eksClusterIamRole,
		},
		PublicSubnetIds: args.publicSubnetIds,
		// RoleMappings: [],
		Tags: pulumi.StringMap{
			"Env":  pulumi.String(args.env),
			"Name": pulumi.String(args.env),
		},
		Version: pulumi.String(args.k8sVersion),
		VpcId:   args.vpcId,
	})
	return cluster, nil
}

func creteEksClusterIamRole(ctx *pulumi.Context, env string) (*iam.Role, error) {
	return iam.NewRole(ctx, env, &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {
					"Service": "eks.amazonaws.com"
					},
					"Action": "sts:AssumeRole"
					}]
					}`)),
	})
}
