package internal

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	gkev1 "google.golang.org/api/container/v1"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
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

func (g *GcpSvs) DeleteObjects(bucketName, prefix string) (error, int) {
	GetLogger().Debug("deleting from bucket: " + bucketName + ", with prefix: " + prefix)
	client, err := storage.NewClient(context.Background())
	if err != nil {
		return err, 0
	}
	bucket := client.Bucket(bucketName)
	itr := bucket.Objects(context.Background(), &storage.Query{Prefix: prefix})
	cnt := 0
	for {
		objAttrs, err := itr.Next()
		if err != nil && err != iterator.Done {
			return err, 0
		}
		if err == iterator.Done {
			break
		}
		if err := bucket.Object(objAttrs.Name).Delete(context.Background()); err != nil {
			return err, 0
		}
		cnt++
	}
	GetLogger().Debug(fmt.Sprintf("deleted <%v> objs", cnt))
	return nil, cnt
}

func (g *GcpSvs) GetK8sConfigFromGke(gkeName string) (*rest.Config, error) {
	cService, err := gkev1.NewService(context.Background())
	if err != nil {
		return nil, err
	}
	// gke, err := cService.Projects.Locations.Clusters.Get(gkeName).Context(context.Background()).Do()
	// if err != nil {
	// 	return nil, err
	// }

	gkes, err := cService.Projects.Zones.Clusters.List(g.ProjectId, "-").Context(context.Background()).Do()
	if err != nil {
		return nil, err
	}
	for _, gke := range gkes.Clusters {
		if gke.Name == gkeName {
			return getK8sClientFromGkeCluster(gke)

		}
	}
	return nil, errors.New("not found")
	// gkeList, err:=cService.Projects.Zones.Clusters.List(g.ProjectId, "-").Context(context.Background()).Do()
	// if err!=nil{return nil,err}
	// for _, gke := range gkeList.Clusters{
	// 	name:= fmt.Sprintf("gke_%s_%s_%s", g.ProjectId, gke.Zone, gke.Name)

	// }

}

// func createK8sClientFromGkeCluster(cluster *gkev1.Cluster) (*rest.Config, error) {
// 	decodedClientCertificate, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClientCertificate)
// 	if err != nil {
// 		return nil, err
// 	}
// 	decodedClientKey, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClientKey)
// 	if err != nil {
// 		return nil, err
// 	}
// 	decodedClusterCaCertificate, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClusterCaCertificate)
// 	if err != nil {
// 		return nil, err
// 	}

// 	config := &rest.Config{
// 		Username: cluster.MasterAuth.Username,
// 		Password: cluster.MasterAuth.Password,
// 		Host:     "https://" + cluster.Endpoint,
// 		TLSClientConfig: rest.TLSClientConfig{
// 			Insecure: false,
// 			CertData: decodedClientCertificate,
// 			KeyData:  decodedClientKey,
// 			CAData:   decodedClusterCaCertificate,
// 		},
// 	}

// 	return config, nil
// }
func getK8sClientFromGkeCluster(c *gkev1.Cluster) (*rest.Config, error) {
	// The cluster CA certificate is base64 encoded from the GKE API.
	rawCaCert, err := base64.URLEncoding.DecodeString(c.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return nil, err
	}

	// This is a low-level structure normally created from parsing a kubeconfig
	// file.  Since we know all values we can create the client object directly.
	//
	// The cluster and user names serve only to define a context that
	// associates login credentials with a specific cluster.
	apiCfg := api.Config{
		Clusters: map[string]*api.Cluster{
			// Define the cluster address and CA Certificate.
			"cluster": {
				Server:                   fmt.Sprintf("https://%s", c.Endpoint),
				InsecureSkipTLSVerify:    false, // Require a valid CA Certificate.
				CertificateAuthorityData: rawCaCert,
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			// Define the user credentials for access to the API.
			"user": {
				AuthProvider: &api.AuthProviderConfig{
					Name: "gcp",
					Config: map[string]string{
						"scopes": "https://www.googleapis.com/auth/cloud-platform",
					},
				},
			},
		},
		Contexts: map[string]*api.Context{
			// Define a context that refers to the above cluster and user.
			"cluster-user": {
				Cluster:  "cluster",
				AuthInfo: "user",
			},
		},
		// Use the above context.
		CurrentContext: "cluster-user",
	}

	// Construct a "direct client" using the auth above to contact the API server.
	defClient := clientcmd.NewDefaultClientConfig(
		apiCfg,
		&clientcmd.ConfigOverrides{
			ClusterInfo: api.Cluster{Server: ""},
		})
	restConfig, err := defClient.ClientConfig()
	if err != nil {
		return nil, err
	}
	return restConfig, nil
}
