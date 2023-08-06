package main

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	awsEks "github.com/pulumi/pulumi-aws/sdk/v5/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi-eks/sdk/go/eks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type CreateEksClusterInput struct {
	amiType               string `yaml:"amiType"`
	desiredSize           int    `yaml:"desiredSize"`
	eksClusterIamRole     *iam.Role
	eksUsers              []string `yaml:"eksUsers"`
	endpointPrivateAccess bool     `yaml:"endpointPrivateAccess"`
	endpointPublicAccess  bool     `yaml:"endpointPublicAccess"`
	env                   string
	initialNodeGroupName  string `yaml:"initialNodeGroupName"`
	instanceType          string `yaml:"instanceType"`
	k8sVersion            string `yaml:"k8sVersion"`
	maxSize               int    `yaml:"maxSize"`
	minSize               int    `yaml:"minSize"`
	publicSubnetIds       pulumi.StringArrayOutput
	privateSubnetIds      pulumi.StringArrayOutput
	vpcId                 pulumi.StringOutput
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		config := config.New(ctx, "")
		amiType := config.Require("amiType")
		deploy := config.RequireBool("deploy")
		desiredSize := config.RequireInt("desiredSize")
		eksUsersValues := config.Require("eksUsers")

		var eksUsers []string
		if err := json.Unmarshal([]byte(eksUsersValues), &eksUsers); err != nil {
			return err
		}
		endpointPrivateAccess := config.RequireBool("endpointPrivateAccess")
		endpointPublicAccess := config.RequireBool("endpointPublicAccess")
		env := config.Require("env")
		instanceType := config.Require("instanceType")
		initialNodeGroupName := config.Require("initialNodeGroupName")
		k8sVersion := config.Require("k8sVersion")
		minSize := config.RequireInt("minSize")
		maxSize := config.RequireInt("maxSize")

		vpcStackRef, err := pulumi.NewStackReference(ctx, fmt.Sprintf("organization/infrastructure-vpc/%s", env), nil)
		if err != nil {
			return err
		}
		vpcId := vpcStackRef.GetStringOutput(pulumi.String("vpcId"))

		privateSubnetIds := vpcStackRef.GetOutput(pulumi.String("privateSubnetIds")).AsStringArrayOutput().ApplyT(func(privateSubnetIds []string) []string {
			return privateSubnetIds
		}).(pulumi.StringArrayOutput)
		publicSubnetIds := vpcStackRef.GetOutput(pulumi.String("publicSubnetIds")).AsStringArrayOutput().ApplyT(func(publicSubnetIds []string) []string {
			return publicSubnetIds
		}).(pulumi.StringArrayOutput)

		var cluster *eks.Cluster
		if deploy {
			eksClusterIamRole, _ := creteEksClusterIamRole(ctx, env)
			cluster, _ = createEksCluster(ctx, &CreateEksClusterInput{
				amiType,
				desiredSize,
				eksClusterIamRole,
				eksUsers,
				endpointPrivateAccess,
				endpointPublicAccess,
				env,
				initialNodeGroupName,
				instanceType,
				k8sVersion,
				maxSize,
				minSize,
				publicSubnetIds,
				privateSubnetIds,
				vpcId,
			})
		}

		ctx.Export("clusterKubeConfig", pulumi.ToSecret(cluster.Core.Kubeconfig()))
		ctx.Export("clusterName", cluster.EksCluster.Name())
		ctx.Export("clusterOidcArn", cluster.Core.OidcProvider().Arn())
		ctx.Export("clusterOidcUrl", cluster.Core.OidcProvider().Url())
		ctx.Export("clusterVersion", cluster.EksCluster.Version())
		ctx.Export("vpcId", vpcId.ApplyT(func(vpcId string) string { return vpcId }))
		ctx.Export("privateSubnetIds", privateSubnetIds)
		ctx.Export("publicSubnetIds", publicSubnetIds)

		return nil
	})
}

func createEksCluster(ctx *pulumi.Context, args *CreateEksClusterInput) (*eks.Cluster, error) {
	cluster, err := eks.NewCluster(ctx, args.env, &eks.ClusterArgs{
		CreateOidcProvider: pulumi.Bool(true),
		DesiredCapacity:    pulumi.Int(args.desiredSize),
		EnabledClusterLogTypes: pulumi.StringArray{
			pulumi.String("api"),
			pulumi.String("audit"),
			pulumi.String("authenticator"),
		},
		EndpointPrivateAccess: pulumi.Bool(true),
		EndpointPublicAccess:  pulumi.Bool(true),
		Fargate:               pulumi.Bool(false),
		InstanceRoles: iam.RoleArray{
			args.eksClusterIamRole,
		},
		InstanceType:         pulumi.String(args.instanceType),
		Name:                 pulumi.String(args.env),
		PrivateSubnetIds:     args.privateSubnetIds,
		PublicSubnetIds:      args.publicSubnetIds,
		RoleMappings:         getRoleMappings(ctx, args.eksUsers),
		SkipDefaultNodeGroup: pulumi.BoolRef(true),
		Tags: pulumi.StringMap{
			"Env":         pulumi.String(args.env),
			"Environment": pulumi.String(args.env),
			"Name":        pulumi.String(args.env),
		},
		Version: pulumi.String(args.k8sVersion),
		VpcId:   args.vpcId,
	})
	if err != nil {
		return nil, err
	}

	_, err = eks.NewManagedNodeGroup(ctx, args.initialNodeGroupName, &eks.ManagedNodeGroupArgs{
		AmiType:       pulumi.String(args.amiType),
		Cluster:       cluster,
		DiskSize:      pulumi.Int(20),
		NodeGroupName: pulumi.String(args.initialNodeGroupName),
		NodeRoleArn:   args.eksClusterIamRole.Arn,
		InstanceTypes: pulumi.StringArray{
			pulumi.String(args.instanceType),
		},
		Labels: pulumi.StringMap{
			"ondemand": pulumi.String("true"),
		},
		ScalingConfig: &awsEks.NodeGroupScalingConfigArgs{
			MinSize:     pulumi.Int(args.minSize),
			MaxSize:     pulumi.Int(args.maxSize),
			DesiredSize: pulumi.Int(args.desiredSize),
		},
		Tags: pulumi.StringMap{
			"Env":         pulumi.String(args.env),
			"Environment": pulumi.String(args.env),
			"Name":        pulumi.String(args.initialNodeGroupName),
		},
	})
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

func getRoleMappings(ctx *pulumi.Context, eksUsers []string) eks.RoleMappingArray {
	accountId, err := aws.GetCallerIdentity(ctx)
	if err != nil {
		return eks.RoleMappingArray{}
	}

	roleMappings := make(eks.RoleMappingArray, len(eksUsers))
	for idx, eksUser := range eksUsers {
		roleMappings[idx] = eks.RoleMappingArgs{
			Groups:   pulumi.ToStringArray([]string{"system:masters"}),
			Username: pulumi.String(eksUser),
			RoleArn:  pulumi.Sprintf("arn:aws:iam::%s:user/%s", accountId.AccountId, eksUser),
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
