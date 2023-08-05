package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/kms"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		kmsKey, err := kms.NewKey(ctx, "myKMSKey", &kms.KeyArgs{
			DeletionWindowInDays: pulumi.Int(7), // Optional
		})
		if err != nil {
			return err
		}

		bucket, err := s3.NewBucket(ctx, "pulumi-state", &s3.BucketArgs{
			ServerSideEncryptionConfiguration: &s3.BucketServerSideEncryptionConfigurationArgs{
				Rule: s3.BucketServerSideEncryptionConfigurationRuleArgs{
					ApplyServerSideEncryptionByDefault: &s3.BucketServerSideEncryptionConfigurationRuleApplyServerSideEncryptionByDefaultArgs{
						SseAlgorithm: pulumi.String("AES256"),
					},
				},
			},
			Tags: pulumi.StringMap{
				"Name": pulumi.String("pulumi-state"),
			},
			Versioning: &s3.BucketVersioningArgs{
				Enabled: pulumi.Bool(true),
			},
		})
		if err != nil {
			return err
		}

		_, err = s3.NewBucketPublicAccessBlock(ctx, "pulumi-state", &s3.BucketPublicAccessBlockArgs{
			Bucket:                bucket.ID(),
			BlockPublicAcls:       pulumi.Bool(true),
			BlockPublicPolicy:     pulumi.Bool(true),
			IgnorePublicAcls:      pulumi.Bool(true),
			RestrictPublicBuckets: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}

		// Export the bucket name and KMS key ID
		ctx.Export("bucketName", bucket.Bucket)
		ctx.Export("kmsKeyId", kmsKey.KeyId)

		return nil
	})
}
