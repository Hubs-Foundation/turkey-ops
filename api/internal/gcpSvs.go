package internal

import (
	"net/http"

	"cloud.google.com/go/compute/metadata"
)

type GcpSvs struct {
	ProjectId string
}

func NewGcpSvs() (*GcpSvs, error) {
	c := metadata.NewClient(&http.Client{})
	pid, err := c.ProjectID()
	if err != nil {
		return nil, err
	}
	return &GcpSvs{
		ProjectId: pid,
	}, nil
}
