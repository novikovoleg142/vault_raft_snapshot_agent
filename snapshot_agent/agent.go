package snapshot_agent

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/novikovoleg142/vault_raft_snapshot_agent/config"

	"cloud.google.com/go/storage"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	vaultApi "github.com/hashicorp/vault/api"
)

type Snapshotter struct {
	API             *vaultApi.Client
	Uploader        *s3manager.Uploader
	S3Client        *s3.S3
	GCPBucket       *storage.BucketHandle
	AzureUploader   azblob.ContainerURL
	TokenExpiration time.Time
}

func NewSnapshotter(config *config.Configuration) (*Snapshotter, error) {
	snapshotter := &Snapshotter{}
	err := snapshotter.ConfigureVaultClient(config)
	if err != nil {
		return nil, err
	}
	if config.AWS.Bucket != "" {
		err = snapshotter.ConfigureS3(config)
		if err != nil {
			return nil, err
		}
	}
	return snapshotter, nil
}

func (s *Snapshotter) ConfigureVaultClient(config *config.Configuration) error {
	vaultConfig := vaultApi.DefaultConfig()
	if config.Address != "" {
		vaultConfig.Address = config.Address
	}
	tlsConfig := &vaultApi.TLSConfig{
		Insecure: true,
	}
	vaultConfig.ConfigureTLS(tlsConfig)
	api, err := vaultApi.NewClient(vaultConfig)
	if err != nil {
		return err
	}
	s.API = api
	if config.VaultAuthMethod == "k8s" {
		return s.SetClientTokenFromK8sAuth(config)
	}
	return s.SetClientTokenFromAppRole(config)
}

func (s *Snapshotter) SetClientTokenFromAppRole(config *config.Configuration) error {
	data := map[string]interface{}{
		"role_id":   config.RoleID,
		"secret_id": config.SecretID,
	}
	approle := "approle"
	if config.Approle != "" {
		approle = config.Approle
	}
	resp, err := s.API.Logical().Write("auth/"+approle+"/login", data)
	if err != nil {
		return fmt.Errorf("error logging into AppRole auth backend: %s", err)
	}
	s.API.SetToken(resp.Auth.ClientToken)
	s.TokenExpiration = time.Now().Add(time.Duration((time.Second * time.Duration(resp.Auth.LeaseDuration)) / 2))
	return nil
}

func (s *Snapshotter) SetClientTokenFromK8sAuth(config *config.Configuration) error {

	if config.K8sAuthPath == "" || config.K8sAuthRole == "" {
		return errors.New("missing k8s auth definitions")
	}

	jwt, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return err
	}
	data := map[string]string{
		"role": config.K8sAuthRole,
		"jwt":  string(jwt),
	}

	login := path.Clean("/v1/auth/" + config.K8sAuthPath + "/login")
	req := s.API.NewRequest("POST", login)
	req.SetJSONBody(data)

	resp, err := s.API.RawRequest(req)
	if err != nil {
		return err
	}
	if respErr := resp.Error(); respErr != nil {
		return respErr
	}

	var result vaultApi.Secret
	if err := resp.DecodeJSON(&result); err != nil {
		return err
	}

	s.API.SetToken(result.Auth.ClientToken)
	s.TokenExpiration = time.Now().Add(time.Duration((time.Second * time.Duration(result.Auth.LeaseDuration)) / 2))
	return nil
}

func (s *Snapshotter) ConfigureS3(config *config.Configuration) error {
	awsConfig := &aws.Config{Region: aws.String(config.AWS.Region)}
	if config.Skip_ssl != false {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		client := &http.Client{Transport: tr} //Create custom transport with disable SSL
		awsConfig.HTTPClient = client
	}

	if config.AWS.AccessKeyID != "" && config.AWS.SecretAccessKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentials(config.AWS.AccessKeyID, config.AWS.SecretAccessKey, "")
	}

	if config.AWS.Endpoint != "" {
		awsConfig.Endpoint = aws.String(config.AWS.Endpoint)
	}

	if config.AWS.S3ForcePathStyle != false {
		awsConfig.S3ForcePathStyle = aws.Bool(config.AWS.S3ForcePathStyle)
	}

	if config.AWS.DisableSSL != false {
		awsConfig.DisableSSL = aws.Bool(config.AWS.DisableSSL)
	}

	sess := session.Must(session.NewSession(awsConfig))
	s.S3Client = s3.New(sess)
	s.Uploader = s3manager.NewUploader(sess)
	return nil
}
