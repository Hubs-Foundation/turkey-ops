package internal

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/iterator"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
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

func (g *GcpSvs) GCS_DeleteObjects(bucketName, prefix string) error {
	GetLogger().Debug("deleting from bucket: " + bucketName + ", with prefix: " + prefix)
	client, err := storage.NewClient(context.Background())
	if err != nil {
		return err
	}
	bucket := client.Bucket(bucketName)
	itr := bucket.Objects(context.Background(), &storage.Query{Prefix: prefix})
	cnt := 0
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
		cnt++
	}
	GetLogger().Debug(fmt.Sprintf("deleted <%v> objs", cnt))
	return nil
}

func (g *GcpSvs) GCS_WriteFile(bucketName, filename, fileContent string) error {
	GetLogger().Debug("writing to bucket: " + bucketName + ", key: " + filename + ", fileContent: " + fileContent)
	client, err := storage.NewClient(context.Background())
	if err != nil {
		return err
	}
	obj := client.Bucket(bucketName).Object(filename)
	w := obj.NewWriter(context.Background())
	_, err = fmt.Fprint(w, fileContent)
	if err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

func (g *GcpSvs) GCS_ReadFile(bucketName, filename string) ([]byte, error) {
	GetLogger().Debug("reading bucket: " + bucketName + ", key: " + filename)
	client, err := storage.NewClient(context.Background())
	if err != nil {
		return nil, err
	}
	obj := client.Bucket(bucketName).Object(filename)
	r, err := obj.NewReader(context.Background())
	if err != nil {
		return nil, err
	}
	defer r.Close()
	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (g *GcpSvs) GCS_List(bucketName, prefix string) ([]string, error) {
	GetLogger().Debug("reading bucket: " + bucketName + ", prefix: " + prefix)
	client, err := storage.NewClient(context.Background())
	if err != nil {
		return nil, err
	}
	query := &storage.Query{
		Prefix: prefix,
		// Delimiter: delimiter,	#buggy on their end
	}
	query.SetAttrSelection([]string{"Name"})
	objItr := client.Bucket(bucketName).Objects(context.Background(), query)
	// names := make(map[string]bool)
	var res []string
	for {
		attrs, err := objItr.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		// names[strings.Split(attrs.Name, "/")[0]] = true
		res = append(res, attrs.Name)
	}
	// for k, _ := range names {
	// 	res = append(res, k)
	// }
	GetLogger().Sugar().Debugf("found %v items", len(res))
	return res, nil
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
	return nil, errors.New("not found (gkeName: " + gkeName + " )")
	// gkeList, err:=cService.Projects.Zones.Clusters.List(g.ProjectId, "-").Context(context.Background()).Do()
	// if err!=nil{return nil,err}
	// for _, gke := range gkeList.Clusters{
	// 	name:= fmt.Sprintf("gke_%s_%s_%s", g.ProjectId, gke.Zone, gke.Name)

	// }

}

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

func (g *GcpSvs) GetSqlIps(InstanceId string) (map[string]string, error) {
	ctx := context.Background()
	sqladminService, err := sqladmin.NewService(ctx)
	if err != nil {
		return nil, err
	}

	instance, err := sqladminService.Instances.Get(g.ProjectId, InstanceId).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	IpMap := make(map[string]string)
	for _, Ip := range instance.IpAddresses {

		IpMap[Ip.Type] = Ip.IpAddress

	}
	GetLogger().Sugar().Debugf("IpMap: %v", IpMap)
	return IpMap, nil
}

//warning -- untested
func (g *GcpSvs) Dns_createRecordSet(zoneName, recSetName, recType string, recSetData []string) error {

	GetLogger().Sugar().Debugf("adding %v:%v:%v to %v", recSetName, recType, recSetData, zoneName)

	ctx := context.Background()
	dnsService, err := dns.NewService(ctx)
	if err != nil {
		return err
	}
	rec := &dns.ResourceRecordSet{
		Name:    recSetName,
		Rrdatas: recSetData,
		Ttl:     int64(60),
		Type:    recType,
	}
	change := &dns.Change{
		Additions: []*dns.ResourceRecordSet{rec},
	}
	_, err = dnsService.Changes.Create(g.ProjectId, zoneName, change).Context(ctx).Do()
	if err != nil {
		return err
	}
	return nil
}
