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
	apiCfg := api.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters:   map[string]*api.Cluster{},  // Clusters is a map of referencable names to cluster configs
		AuthInfos:  map[string]*api.AuthInfo{}, // AuthInfos is a map of referencable names to user configs
		Contexts:   map[string]*api.Context{},  // Contexts is a map of referencable names to context configs
	}
	// name := fmt.Sprintf("gke_%s_%s_%s", projectId, f.Zone, f.Name)
	name := c.Name
	cert, err := base64.StdEncoding.DecodeString(c.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return nil, fmt.Errorf("invalid certificate cluster=%s cert=%s: %w", name, c.MasterAuth.ClusterCaCertificate, err)
	}
	// example: gke_my-project_us-central1-b_cluster-1 => https://XX.XX.XX.XX
	apiCfg.Clusters[name] = &api.Cluster{
		CertificateAuthorityData: cert,
		Server:                   "https://" + c.Endpoint,
	}
	// Just reuse the context name as an auth name.
	apiCfg.Contexts[name] = &api.Context{
		Cluster:  name,
		AuthInfo: name,
	}
	// GCP specific configation; use cloud platform scope.
	apiCfg.AuthInfos[name] = &api.AuthInfo{
		AuthProvider: &api.AuthProviderConfig{
			Name: "gcp",
			Config: map[string]string{
				"scopes": "https://www.googleapis.com/auth/cloud-platform",
			},
		},
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
