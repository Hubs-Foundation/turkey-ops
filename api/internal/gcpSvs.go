package internal

import (
	"context"

	"golang.org/x/oauth2/google"
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
