package aws

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

// Region is a curated AWS region.
type Region struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// InstanceType is a curated EC2 instance type.
type InstanceType struct {
	ID          string  `json:"id"`
	VCPUs       int     `json:"vcpus"`
	MemoryGiB   float64 `json:"memory_gib"`
	Description string  `json:"description"`
}

// OSOption represents an OS choice for EC2.
type OSOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListRegions returns a curated list of AWS regions.
func ListRegions() []Region {
	return []Region{
		{ID: "us-east-1", Name: "US East (N. Virginia)"},
		{ID: "us-east-2", Name: "US East (Ohio)"},
		{ID: "us-west-1", Name: "US West (N. California)"},
		{ID: "us-west-2", Name: "US West (Oregon)"},
		{ID: "eu-west-1", Name: "Europe (Ireland)"},
		{ID: "eu-west-2", Name: "Europe (London)"},
		{ID: "eu-central-1", Name: "Europe (Frankfurt)"},
		{ID: "ap-southeast-1", Name: "Asia Pacific (Singapore)"},
		{ID: "ap-southeast-2", Name: "Asia Pacific (Sydney)"},
		{ID: "ap-northeast-1", Name: "Asia Pacific (Tokyo)"},
	}
}

// ListInstanceTypes returns a curated list of EC2 instance types.
func ListInstanceTypes() []InstanceType {
	return []InstanceType{
		{ID: "t3.micro", VCPUs: 2, MemoryGiB: 1, Description: "t3.micro — 2 vCPU / 1 GiB RAM"},
		{ID: "t3.small", VCPUs: 2, MemoryGiB: 2, Description: "t3.small — 2 vCPU / 2 GiB RAM"},
		{ID: "t3.medium", VCPUs: 2, MemoryGiB: 4, Description: "t3.medium — 2 vCPU / 4 GiB RAM"},
		{ID: "t3.large", VCPUs: 2, MemoryGiB: 8, Description: "t3.large — 2 vCPU / 8 GiB RAM"},
		{ID: "t3.xlarge", VCPUs: 4, MemoryGiB: 16, Description: "t3.xlarge — 4 vCPU / 16 GiB RAM"},
	}
}

// ListOSOptions returns the supported OS options.
func ListOSOptions() []OSOption {
	return []OSOption{
		{ID: "ubuntu-22.04", Name: "Ubuntu 22.04 LTS"},
		{ID: "ubuntu-24.04", Name: "Ubuntu 24.04 LTS"},
		{ID: "amazon-linux-2023", Name: "Amazon Linux 2023"},
	}
}

// DefaultUsername returns the default SSH username for a given OS ID.
func DefaultUsername(osID string) string {
	switch osID {
	case "amazon-linux-2023":
		return "ec2-user"
	default:
		return "ubuntu"
	}
}

// GetAMI resolves an AMI ID for the given region and OS using SSM Parameter Store.
func GetAMI(ctx context.Context, cfg aws.Config, region, osID string) (string, error) {
	// Use region-specific config for SSM
	regionCfg := cfg.Copy()
	regionCfg.Region = region
	ssmClient := ssm.NewFromConfig(regionCfg)

	var paramPath string
	switch osID {
	case "ubuntu-22.04":
		paramPath = "/aws/service/canonical/ubuntu/server/22.04/stable/current/amd64/hvm/ebs-gp2/ami-id"
	case "ubuntu-24.04":
		paramPath = "/aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id"
	case "amazon-linux-2023":
		paramPath = "/aws/service/ami-amazon-linux-latest/al2023-ami-hvm-x86_64-gp2"
	default:
		return "", fmt.Errorf("unsupported OS: %s", osID)
	}

	out, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(paramPath),
	})
	if err != nil {
		return "", fmt.Errorf("get AMI from SSM for %s/%s: %w", region, osID, err)
	}
	return aws.ToString(out.Parameter.Value), nil
}

// generateED25519KeyPair generates an ed25519 key pair and returns
// (privateKeyPEM, publicKeyOpenSSH, error).
func generateED25519KeyPair() (privateKeyPEM string, publicKeyOpenSSH string, err error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ed25519 key: %w", err)
	}

	privPEM, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return "", "", fmt.Errorf("marshal private key: %w", err)
	}
	privateKeyPEM = string(pem.EncodeToMemory(privPEM))

	sshPub, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return "", "", fmt.Errorf("create ssh public key: %w", err)
	}
	publicKeyOpenSSH = string(ssh.MarshalAuthorizedKey(sshPub))
	return privateKeyPEM, publicKeyOpenSSH, nil
}

// ensureSecurityGroup finds or creates the "localisprod-ssh" security group in the default VPC.
func ensureSecurityGroup(ctx context.Context, ec2Client *ec2.Client) (string, error) {
	// Find default VPC
	vpcOut, err := ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("isDefault"), Values: []string{"true"}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("describe default VPC: %w", err)
	}
	if len(vpcOut.Vpcs) == 0 {
		return "", fmt.Errorf("no default VPC found in region")
	}
	vpcID := aws.ToString(vpcOut.Vpcs[0].VpcId)

	// Check if security group already exists
	sgOut, err := ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("group-name"), Values: []string{"localisprod-ssh"}},
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("describe security groups: %w", err)
	}
	if len(sgOut.SecurityGroups) > 0 {
		return aws.ToString(sgOut.SecurityGroups[0].GroupId), nil
	}

	// Create security group
	createOut, err := ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String("localisprod-ssh"),
		Description: aws.String("localisprod SSH access"),
		VpcId:       aws.String(vpcID),
	})
	if err != nil {
		return "", fmt.Errorf("create security group: %w", err)
	}
	sgID := aws.ToString(createOut.GroupId)

	// Add SSH ingress rule
	_, err = ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(sgID),
		IpPermissions: []ec2types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(22),
				ToPort:     aws.Int32(22),
				IpRanges: []ec2types.IpRange{
					{CidrIp: aws.String("0.0.0.0/0")},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("authorize ssh ingress: %w", err)
	}

	return sgID, nil
}

// ProvisionInstance creates an EC2 instance, waits for it to be running,
// and returns (host IP, instance ID, private key PEM, error).
func ProvisionInstance(ctx context.Context, accessKey, secretKey, region, instanceType, osID, name string) (host, instanceID, privateKeyPEM string, err error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return "", "", "", fmt.Errorf("load AWS config: %w", err)
	}

	ec2Client := ec2.NewFromConfig(cfg)

	privPEM, pubKey, err := generateED25519KeyPair()
	if err != nil {
		return "", "", "", err
	}

	// Import key pair
	keyName := "localisprod-" + uuid.New().String()[:8]
	_, err = ec2Client.ImportKeyPair(ctx, &ec2.ImportKeyPairInput{
		KeyName:           aws.String(keyName),
		PublicKeyMaterial: []byte(pubKey),
	})
	if err != nil {
		return "", "", "", fmt.Errorf("import key pair: %w", err)
	}
	// Cleanup key pair after provisioning
	defer func() {
		_, _ = ec2Client.DeleteKeyPair(context.Background(), &ec2.DeleteKeyPairInput{
			KeyName: aws.String(keyName),
		})
	}()

	// Ensure security group
	sgID, err := ensureSecurityGroup(ctx, ec2Client)
	if err != nil {
		return "", "", "", err
	}

	// Get AMI ID
	amiID, err := GetAMI(ctx, cfg, region, osID)
	if err != nil {
		return "", "", "", err
	}

	// Launch instance
	runOut, err := ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:          aws.String(amiID),
		InstanceType:     ec2types.InstanceType(instanceType),
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		KeyName:          aws.String(keyName),
		SecurityGroupIds: []string{sgID},
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeInstance,
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String(name)},
					{Key: aws.String("ManagedBy"), Value: aws.String("localisprod")},
				},
			},
		},
	})
	if err != nil {
		return "", "", "", fmt.Errorf("run instances: %w", err)
	}
	if len(runOut.Instances) == 0 {
		return "", "", "", fmt.Errorf("no instance returned from RunInstances")
	}

	ec2InstanceID := aws.ToString(runOut.Instances[0].InstanceId)

	// Poll until running (max 5 minutes)
	deadline := time.Now().Add(5 * time.Minute)
	var publicIP string
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", "", "", ctx.Err()
		default:
		}

		descOut, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: []string{ec2InstanceID},
		})
		if err != nil {
			return "", "", "", fmt.Errorf("describe instance: %w", err)
		}
		if len(descOut.Reservations) > 0 && len(descOut.Reservations[0].Instances) > 0 {
			inst := descOut.Reservations[0].Instances[0]
			if inst.State != nil && inst.State.Name == ec2types.InstanceStateNameRunning {
				if inst.PublicIpAddress != nil {
					publicIP = aws.ToString(inst.PublicIpAddress)
					break
				}
			}
		}
		time.Sleep(10 * time.Second)
	}

	if publicIP == "" {
		return "", "", "", fmt.Errorf("instance did not become running within 5 minutes")
	}

	return publicIP, ec2InstanceID, privPEM, nil
}
