package internal

import (
	"context"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
)

type GcpSvs struct {
	ProjectId string
}

func NewGcpSvs() (*GcpSvs, error) {

	creds, err := google.FindDefaultCredentials(context.Background())
	if err != nil {
		return nil, err
	}
	return &GcpSvs{
		ProjectId: creds.ProjectID,
	}, nil
}

func (g *GcpSvs) DeleteObjects(bucketName, prefix string) error {
	client, err := storage.NewClient(context.Background())
	if err != nil {
		return err
	}
	bucket := client.Bucket(bucketName)
	itr := bucket.Objects(context.Background(), &storage.Query{Prefix: prefix})
	for {
		objAttrs, err := itr.Next()
		if err != nil && err != iterator.Done {
			return err
		}
		if err == iterator.Done {
			break
		}
		if err := bucket.Object(objAttrs.Name).Delete(context.Background()); err != nil {
			return err
		}
	}
	return nil
}
