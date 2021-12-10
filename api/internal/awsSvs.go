package internal

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
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

func (as AwsSvs) DownloadS3item(bucket, key string, f *os.File) error {
	downloader := s3manager.NewDownloader(as.Sess)
	_, err := downloader.Download(
		f,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	if err != nil {
		fmt.Println("---DownloadS3item---err: " + err.Error())
		return err
	}
	return nil
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
