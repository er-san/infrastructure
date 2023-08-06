package main

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi-eks/sdk/go/eks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type CreateEksClusterInput struct {
	ctx               *pulumi.Context
	DesiredCapacity   int
	InstanceType      string
	MaxSize           int
	MinSize           int
	EksClusterIamRole *iam.Role
	Env               string
	K8sVersion        string
	PublicSubnetIds   pulumi.StringArrayOutput
	PrivateSubnetIds  pulumi.StringArrayOutput
	VpcId             pulumi.StringOutput
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		config := config.New(ctx, "")
		env := config.Require("env")
		k8sVersion := config.Require("k8sVersion")
		defaultDesiredCapacity := config.RequireInt("defaultDesiredCapacity")
		instanceType := config.Require("instanceType")
		defaultMinSize := config.RequireInt("defaultMinSize")
		defaultMaxSize := config.RequireInt("defaultMaxSize")

		vpcStackRef, err := pulumi.NewStackReference(ctx, fmt.Sprintf("organization/infrastructure-vpc/%s", env), nil)
		if err != nil {
			return err
		}

		vpcId := vpcStackRef.GetStringOutput(pulumi.String("vpcId"))

		publicSubnetIds := vpcStackRef.GetOutput(pulumi.String("publicSubnetIds")).AsStringArrayOutput().ApplyT(func(publicSubnetIds []string) []string {
			return publicSubnetIds
		}).(pulumi.StringArrayOutput)

		privateSubnetIds := vpcStackRef.GetOutput(pulumi.String("privateSubnetIds")).AsStringArrayOutput().ApplyT(func(privateSubnetIds []string) []string {
			return privateSubnetIds
		}).(pulumi.StringArrayOutput)

		deploy := config.RequireBool("deploy")
		var cluster *eks.Cluster
		if deploy {
			eksClusterIamRole, _ := creteEksClusterIamRole(ctx, env)
			cluster, _ = createEksCluster(CreateEksClusterInput{
				ctx,
				defaultDesiredCapacity,
				instanceType,
				defaultMaxSize,
				defaultMinSize,
				eksClusterIamRole,
				env,
				k8sVersion,
				publicSubnetIds,
				privateSubnetIds,
				vpcId,
			})
		}

		ctx.Export("clusterName", cluster.EksCluster.Name())
		ctx.Export("vpcId", vpcId.ApplyT(func(vpcId string) string { return vpcId }))
		ctx.Export("publicSubnetIds", publicSubnetIds)

		return nil
	})
}

// func createDefaultLaunchTemplate(ctx *pulumi.Context, env string, nodeSecurityGroup *ec2.SecurityGroup) (*ec2.LaunchTemplate, error) {
// 	return ec2.NewLaunchTemplate(ctx, fmt.Sprintf("%s-default-node-group", env), &ec2.LaunchTemplateArgs{
// 		BlockDeviceMappings: ec2.LaunchTemplateBlockDeviceMappingArray{
// 			ec2.LaunchTemplateBlockDeviceMappingArgs{
// 				DeviceName: pulumi.String("/dev/xvda"),
// 				Ebs: ec2.LaunchTemplateBlockDeviceMappingEbsArgs{
// 					VolumeSize: pulumi.Int(20),
// 				},
// 			},
// 		},
// 		Name: pulumi.String(fmt.Sprintf("%s-default-node-group", env)),
// 		VpcSecurityGroupIds: pulumi.StringArray{
// 			nodeSecurityGroup.ID(),
// 		},
// 	})
// }

func createEksCluster(args CreateEksClusterInput) (*eks.Cluster, error) {
	cluster, _ := eks.NewCluster(args.ctx, args.Env, &eks.ClusterArgs{
		CreateOidcProvider: pulumi.Bool(true),
		DesiredCapacity:    pulumi.Int(args.DesiredCapacity),
		EnabledClusterLogTypes: pulumi.StringArray{
			pulumi.String("api"),
			pulumi.String("audit"),
			pulumi.String("authenticator"),
		},
		EndpointPrivateAccess: pulumi.Bool(true),
		EndpointPublicAccess:  pulumi.Bool(false),
		Fargate:               pulumi.Bool(false),
		InstanceRoles: iam.RoleArray{
			args.EksClusterIamRole,
		},
		InstanceType:         pulumi.String(args.InstanceType),
		Name:                 pulumi.String(args.Env),
		PrivateSubnetIds:     args.PrivateSubnetIds,
		PublicSubnetIds:      args.PublicSubnetIds,
		RoleMappings:         getRoleMappings(args.ctx),
		SkipDefaultNodeGroup: pulumi.BoolRef(false),
		Tags: pulumi.StringMap{
			"Env":         pulumi.String(args.Env),
			"Environment": pulumi.String(args.Env),
			"Name":        pulumi.String(args.Env),
		},
		Version: pulumi.String(args.K8sVersion),
		VpcId:   args.VpcId,
	})
	return cluster, nil
}

func getRoleMappings(ctx *pulumi.Context) eks.RoleMappingArray {
	config := config.New(ctx, "")
	eksUsersValues := config.Require("eksUsers")

	var eksUsers []string
	if err := json.Unmarshal([]byte(eksUsersValues), &eksUsers); err != nil {
		return eks.RoleMappingArray{}
	}

	accountId, err := aws.GetCallerIdentity(ctx)
	if err != nil {
		return eks.RoleMappingArray{}
	}

	roleMappings := make(eks.RoleMappingArray, len(eksUsers))
	for idx, eksUser := range eksUsers {
		roleMappings[idx] = eks.RoleMappingArgs{
			Groups:   pulumi.ToStringArray([]string{"system:masters"}),
			RoleArn:  pulumi.Sprintf("arn:aws:iam::%s:user/%s", accountId.AccountId, eksUser),
			Username: pulumi.String(eksUser),
		}
	}
	return roleMappings
}

func creteEksClusterIamRole(ctx *pulumi.Context, env string) (*iam.Role, error) {
	return iam.NewRole(ctx, fmt.Sprintf("%s-eks-cluster-role", env), &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
			  {
				"Effect": "Allow",
				"Principal": {
				  "Service": "ec2.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			  }
			]
		  }`)),
		ManagedPolicyArns: pulumi.StringArray{
			pulumi.String("arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"),
			pulumi.String("arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"),
			pulumi.String("arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"),
		},
	})
}

// func createEksClusterInstanceProfile(ctx *pulumi.Context, env string) (*iam.InstanceProfile, error) {
// 	role, _ := iam.NewRole(ctx, fmt.Sprintf("%s-eks-cluster-default-node-role", env), &iam.RoleArgs{
// 		AssumeRolePolicy: pulumi.String(fmt.Sprintf(`{
// 			"Version": "2012-10-17",
// 			"Statement": [{
// 				"Effect": "Allow",
// 				"Principal": {
// 					"Service": "eks.amazonaws.com"
// 					},
// 					"Action": "sts:AssumeRole"
// 					}]
// 		}`)),
// 	})
// 	return iam.NewInstanceProfile(ctx, fmt.Sprintf("%s-eks-default-node-group-instance-profile", env), &iam.InstanceProfileArgs{
// 		Role: role.Arn,
// 	})
// }
