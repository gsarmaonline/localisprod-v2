package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func newEC2Client(ctx context.Context, accessKey, secretKey, region string) (*ec2.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return ec2.NewFromConfig(cfg), nil
}

// getInstanceAZ returns the availability zone of an EC2 instance.
func getInstanceAZ(ctx context.Context, client *ec2.Client, instanceID string) (string, error) {
	out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return "", fmt.Errorf("describe instance %s: %w", instanceID, err)
	}
	if len(out.Reservations) == 0 || len(out.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("instance %s not found", instanceID)
	}
	az := aws.ToString(out.Reservations[0].Instances[0].Placement.AvailabilityZone)
	if az == "" {
		return "", fmt.Errorf("instance %s has no availability zone", instanceID)
	}
	return az, nil
}

// CreateVolume creates an EBS gp3 volume in the same AZ as the given EC2 instance.
// Returns (volumeID, devicePath, error). Device path for attachment is always /dev/xvdf.
func CreateVolume(ctx context.Context, accessKey, secretKey, region, instanceID string, sizeGB int) (string, string, error) {
	client, err := newEC2Client(ctx, accessKey, secretKey, region)
	if err != nil {
		return "", "", err
	}

	az, err := getInstanceAZ(ctx, client, instanceID)
	if err != nil {
		return "", "", err
	}

	out, err := client.CreateVolume(ctx, &ec2.CreateVolumeInput{
		AvailabilityZone: aws.String(az),
		Size:             aws.Int32(int32(sizeGB)),
		VolumeType:       ec2types.VolumeTypeGp3,
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeVolume,
				Tags: []ec2types.Tag{
					{Key: aws.String("ManagedBy"), Value: aws.String("localisprod")},
				},
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("create EBS volume: %w", err)
	}

	volumeID := aws.ToString(out.VolumeId)

	// Wait for volume to be available (max 3 minutes)
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		default:
		}
		desc, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			VolumeIds: []string{volumeID},
		})
		if err != nil {
			return "", "", fmt.Errorf("describe volume: %w", err)
		}
		if len(desc.Volumes) > 0 && desc.Volumes[0].State == ec2types.VolumeStateAvailable {
			break
		}
		time.Sleep(5 * time.Second)
	}

	return volumeID, "/dev/xvdf", nil
}

// AttachVolume attaches an EBS volume to an EC2 instance as /dev/xvdf and waits for attachment.
func AttachVolume(ctx context.Context, accessKey, secretKey, region, volumeID, instanceID string) error {
	client, err := newEC2Client(ctx, accessKey, secretKey, region)
	if err != nil {
		return err
	}

	_, err = client.AttachVolume(ctx, &ec2.AttachVolumeInput{
		Device:     aws.String("/dev/xvdf"),
		InstanceId: aws.String(instanceID),
		VolumeId:   aws.String(volumeID),
	})
	if err != nil {
		return fmt.Errorf("attach EBS volume: %w", err)
	}

	// Wait for attachment to complete (max 2 minutes)
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		desc, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			VolumeIds: []string{volumeID},
		})
		if err != nil {
			return fmt.Errorf("describe volume: %w", err)
		}
		if len(desc.Volumes) > 0 && len(desc.Volumes[0].Attachments) > 0 {
			state := desc.Volumes[0].Attachments[0].State
			if state == ec2types.VolumeAttachmentStateAttached {
				return nil
			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("volume attachment did not complete within 2 minutes")
}

// DetachVolume force-detaches an EBS volume and waits until it is available again.
func DetachVolume(ctx context.Context, accessKey, secretKey, region, volumeID string) error {
	client, err := newEC2Client(ctx, accessKey, secretKey, region)
	if err != nil {
		return err
	}

	_, err = client.DetachVolume(ctx, &ec2.DetachVolumeInput{
		VolumeId: aws.String(volumeID),
		Force:    aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("detach EBS volume: %w", err)
	}

	// Wait for volume to be available again (max 3 minutes)
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		desc, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			VolumeIds: []string{volumeID},
		})
		if err != nil {
			return fmt.Errorf("describe volume: %w", err)
		}
		if len(desc.Volumes) > 0 && desc.Volumes[0].State == ec2types.VolumeStateAvailable {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("volume detachment did not complete within 3 minutes")
}

// DeleteVolume deletes an EBS volume.
func DeleteVolume(ctx context.Context, accessKey, secretKey, region, volumeID string) error {
	client, err := newEC2Client(ctx, accessKey, secretKey, region)
	if err != nil {
		return err
	}
	_, err = client.DeleteVolume(ctx, &ec2.DeleteVolumeInput{
		VolumeId: aws.String(volumeID),
	})
	if err != nil {
		return fmt.Errorf("delete EBS volume: %w", err)
	}
	return nil
}
