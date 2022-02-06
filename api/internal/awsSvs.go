package internal

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

type AwsSvs struct {
	Sess *session.Session
}

func NewAwsSvs(key, secret, region string) (*AwsSvs, error) {
	os.Setenv("AWS_ACCESS_KEY_ID", key)
	os.Setenv("AWS_SECRET_ACCESS_KEY", secret)
	os.Setenv("AWS_DEFAULT_REGION", region)
	os.Setenv("AWS_REGION", region)
	sess, err := session.NewSession()
	// sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})

	if err != nil {
		return &AwsSvs{}, err
	}
	return &AwsSvs{
		Sess: sess,
	}, err
}

func BuildCFlink(region, stackID string) string {
	return "https://" + region + ".console.aws.amazon.com/cloudformation/home?region=" + region + "#/stacks/stackinfo?stackId=" + stackID
}

func (as AwsSvs) CreateCFstack(stackName string, templateURL string, params []*cloudformation.Parameter, tags []*cloudformation.Tag) error {

	svc := cloudformation.New(as.Sess)

	input := &cloudformation.CreateStackInput{
		TemplateURL:  aws.String(templateURL),
		StackName:    aws.String(stackName),
		Capabilities: []*string{aws.String("CAPABILITY_NAMED_IAM")},
		Parameters:   params,
		Tags:         tags,
	}
	_, err := svc.CreateStack(input)
	if err != nil {
		// Logger.Println("[ERROR]: CreateCFstack FAILED and error = " + err.Error())
		return err
	}
	// Logger.Println("===CreateCFstack started for stackName = " + stackName)

	desInput := &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)}

	return svc.WaitUntilStackCreateComplete(desInput)
}

func (as AwsSvs) GetStackEvents(stackName string) ([]*cloudformation.StackEvent, error) {
	if stackName == "" {
		return nil, errors.New("missing <stackName>")
	}
	svc := cloudformation.New(as.Sess)
	out, err := svc.DescribeStackEvents(&cloudformation.DescribeStackEventsInput{StackName: aws.String(stackName)})
	if err != nil {
		return nil, err
	}
	return out.StackEvents, err
}
func (as AwsSvs) GetStack(stackName string) ([]*cloudformation.Stack, error) {
	if stackName == "" {
		return nil, errors.New("missing <stackName>")
	}
	svc := cloudformation.New(as.Sess)
	out, err := svc.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, err
	}
	return out.Stacks, err
}

func (as AwsSvs) CreateSSMparameter(paramName, paramValue string) error {

	svc := ssm.New(as.Sess)

	req, _ := svc.PutParameterRequest(&ssm.PutParameterInput{
		Name:      aws.String(paramName),
		Overwrite: aws.Bool(true),
		Type:      aws.String("SecureString"),
		Value:     aws.String(paramValue),
	})
	return req.Send()

}

func (as AwsSvs) S3WaitForBucket(bucket string, timeoutSec int) error {
	var err = errors.New("fake error")
	s3svc := s3.New(as.Sess)
	waited := 0
	for err != nil && waited < timeoutSec {
		_, err = s3svc.HeadBucket(&s3.HeadBucketInput{
			Bucket: aws.String(bucket),
		})
		time.Sleep(time.Second * 30)
		waited += 30
	}
	if waited >= timeoutSec {
		return errors.New("timeout --> S3WaitForBucket: " + bucket)
	}
	return nil
}

func (as AwsSvs) S3Download_file(bucket, key string, f *os.File) error {
	downloader := s3manager.NewDownloader(as.Sess)
	_, err := downloader.Download(
		f,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	if err != nil {
		fmt.Println("---DownloadS3item---failed ~~~ bucket: " + bucket + ", key: " + key + ", error: " + err.Error())
		return err
	}
	return nil
}
func (as AwsSvs) S3Download_string(bucket, key string) (string, error) {
	f, _ := ioutil.TempFile("./", "hubs-tmp-")
	err := as.S3Download_file(bucket, key, f)
	if err != nil {
		return "", err
	}
	fBytes, err := ioutil.ReadFile(f.Name())
	if err != nil {
		return "", err
	}
	return string(fBytes), nil
}

func (as AwsSvs) S3UploadFile(f *os.File, bucket, bktKey string) error {
	uploader := s3manager.NewUploader(as.Sess)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(bktKey),
		Body:   f,
	})
	if err != nil {
		fmt.Println("---UploadS3item---err: " + err.Error())
		return fmt.Errorf("failed to upload file, %v", err)
	}
	return nil
}

func (as AwsSvs) S3UploadMimeObj(f *os.File, bucket, bktKey, contentType string) error {
	uploader := s3manager.NewUploader(as.Sess)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(bktKey),
		Body:        f,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		fmt.Println("---UploadS3item---err: " + err.Error())
		return fmt.Errorf("failed to upload file, %v", err)
	}
	return nil
}

func (as AwsSvs) GetAccountID() (string, error) {
	svc := sts.New(as.Sess)
	r, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	return *r.Account, nil
}

func (as AwsSvs) CheckCERTarn(CERTarn string) (cert string, err error) {
	svc := acm.New(as.Sess)
	certOut, err := svc.GetCertificate(&acm.GetCertificateInput{CertificateArn: aws.String(CERTarn)})
	if err != nil {
		return "", err
	}
	return *certOut.Certificate, err
}

func (as AwsSvs) GetK8sConfigFromEks(eksName string) (*rest.Config, error) {
	eksSvc := eks.New(as.Sess)
	res, err := eksSvc.DescribeCluster(&eks.DescribeClusterInput{
		Name: aws.String(eksName),
	})
	if err != nil {
		GetLogger().Error("Error calling DescribeCluster: " + err.Error())
		return nil, err
	}
	k8sCfg, err := newK8sConfigFromEks(res.Cluster)
	if err != nil {
		GetLogger().Error("Error creating clientset: " + err.Error())
		return nil, err
	}
	return k8sCfg, nil
}

func newK8sConfigFromEks(cluster *eks.Cluster) (*rest.Config, error) {
	log.Printf("%+v", cluster)
	gen, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, err
	}
	opts := &token.GetTokenOptions{
		ClusterID: aws.StringValue(cluster.Name),
	}
	tok, err := gen.GetWithOptions(opts)
	if err != nil {
		return nil, err
	}
	ca, err := base64.StdEncoding.DecodeString(aws.StringValue(cluster.CertificateAuthority.Data))
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &rest.Config{
		Host:        aws.StringValue(cluster.Endpoint),
		BearerToken: tok.Token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: ca,
		},
	}, nil
}

func (as AwsSvs) ACM_findCertByDomainName(domainName string, status string) (string, error) {
	acmClient := acm.New(as.Sess)

	findings, err := acmClient.ListCertificates(&acm.ListCertificatesInput{
		CertificateStatuses: aws.StringSlice([]string{status}),
		// MaxItems:            aws.Int64(1000),
	})
	if err != nil {
		return "", err
	}
	GetLogger().Sugar().Debugf("len(findings.CertificateSummaryList): %v", len(findings.CertificateSummaryList))
	if findings.NextToken != nil {
		GetLogger().Warn("acmClient.ListCertificates didn't return all certs in a simple call, consider implement pagination")
	}

	for _, cert := range findings.CertificateSummaryList {
		if *cert.DomainName == domainName {
			return *cert.CertificateArn, nil
		}
	}
	return "", errors.New("not found")
}

func (as AwsSvs) Route53_addRecord(recName, recType, value string) error {

	recNameArr := strings.Split(recName, ".")
	if len(recNameArr) < 2 {
		return errors.New("bad recName: " + recName)
	}
	domain := recNameArr[len(recNameArr)-2] + "." + recNameArr[len(recNameArr)-1] + "."

	r53Client := route53.New(as.Sess)
	hostedZones, err := r53Client.ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
		DNSName: aws.String(domain),
	})
	if err != nil {
		return err
	}
	HostedZoneId := ""
	for _, hz := range hostedZones.HostedZones {
		GetLogger().Debug("dumping: " + *hz.Name)
		if *hz.Name == domain {
			HostedZoneId = *hz.Id
			break
		}
	}
	if HostedZoneId == "" {
		return errors.New("not found: hostedZones.HostedZoneId for " + recName + " using DNSName: " + domain)
	}
	params := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{ // Required
			Changes: []*route53.Change{ // Required
				{ // Required
					Action: aws.String("UPSERT"), // Required
					ResourceRecordSet: &route53.ResourceRecordSet{ // Required
						Name: aws.String(recName), // Required
						Type: aws.String(recType), // Required
						ResourceRecords: []*route53.ResourceRecord{
							{ // Required
								Value: aws.String(value), // Required
							},
						},
						// TTL:           aws.Int64(TTL),
						// Weight:        aws.Int64(weight),
						// SetIdentifier: aws.String("Arbitrary Id describing this change set"),
					},
				},
			},
			// Comment: aws.String("Sample update."),
		},
		HostedZoneId: aws.String(HostedZoneId), // Required
	}
	_, err = r53Client.ChangeResourceRecordSets(params)

	if err != nil {
		return err
	}
	return nil
}
