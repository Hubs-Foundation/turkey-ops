package internal

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/iterator"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	gkev1 "google.golang.org/api/container/v1"

	filestore "cloud.google.com/go/filestore/apiv1"
	filestorepb "cloud.google.com/go/filestore/apiv1/filestorepb"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type GcpSvs struct {
	ProjectId string
	// SAEmail   string
}

func NewGcpSvs() (*GcpSvs, error) {

	creds, err := google.FindDefaultCredentials(context.Background())
	if err != nil {
		return nil, err
	}
	// jsonFile, err := os.Open(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	// if err != nil {
	// 	return nil, err
	// }
	// defer jsonFile.Close()
	// bytes, _ := ioutil.ReadAll(jsonFile)
	// m := make(map[string]string)
	// json.Unmarshal(bytes, &m)

	return &GcpSvs{
		ProjectId: creds.ProjectID,
		// SAEmail:   m["client_email"],
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

func (g *GcpSvs) GCS_makeSignedURL(bucketName, objName, method string) (string, error) {
	GetLogger().Debug("GCS_makeSignedURL: " + bucketName + ", key: " + objName)
	client, err := storage.NewClient(context.Background())
	if err != nil {
		return "", err
	}

	exp := time.Now().Add(7 * 24 * time.Hour)
	// url, err := client.SignedURL(bucketName, objName,
	// 	&storage.SignedURLOptions{
	// 		GoogleAccessID: g.SAEmail,
	// 		Method:         method,
	// 		Expires:        time.Now().Add(7 * 24 * time.Hour),
	// 	})
	// if err != nil {
	// 	fmt.Printf("Error: %v", err)
	// 	return "", err
	// }    storageClient, _ := storage.NewClient(ctx)

	url, err := client.Bucket(bucketName).SignedURL(objName, &storage.SignedURLOptions{
		Method:  method,
		Expires: exp,
	})

	return url, err
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

func (g *GcpSvs) PubSub_PublishMsg(topic string, data []byte) error {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, g.ProjectId)
	if err != nil {
		return err
	}
	msg := &pubsub.Message{
		Data: data,
	}

	res := client.Topic(topic).Publish(ctx, msg)
	_, err = res.Get(ctx)

	return err
}

func (g *GcpSvs) PubSub_Pulling(subscriptionName string, f func(ctx context.Context, msg *pubsub.Message)) error {
	Logger.Debug("pulling subscription: " + subscriptionName)
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, g.ProjectId)
	if err != nil {
		Logger.Error(err.Error())
		return err
	}
	sub := client.Subscription(subscriptionName)
	err = sub.Receive(ctx, f)
	if err != nil {
		Logger.Error(err.Error())
	}

	return err
}

func (g *GcpSvs) Filestore_GetIP(name, location string) (string, error) {
	ctx := context.Background()
	client, err := filestore.NewCloudFilestoreManagerClient(ctx)
	if err != nil {
		return "", err
	}

	req := &filestorepb.GetInstanceRequest{
		Name: `projects/` + g.ProjectId + `/locations/` + location + `/instances/` + name,
	}
	fs, err := client.GetInstance(ctx, req)
	if err != nil {
		return "", err
	}
	GetLogger().Sugar().Debugf("fs: %v", fs)

	ip := fs.Networks[0].IpAddresses[0]
	GetLogger().Debug("Filestore_GetIP: " + ip)
	return ip, err
}

func (g *GcpSvs) FindTandemCidr(vpcName string) (string, error) {
	computeService, err := compute.NewService(context.Background())
	if err != nil {
		Logger.Error(err.Error())
		return "", err
	}

	req := computeService.Subnetworks.AggregatedList(g.ProjectId)
	req.Filter(fmt.Sprintf("network eq .*%s", vpcName))
	subnetList, err := req.Do()
	if err != nil {
		Logger.Error(err.Error())
		return "", err
	}
	// Store the existing CIDRs
	var existingCIDRs []string
	for _, subnetList := range subnetList.Items {
		for _, subnetwork := range subnetList.Subnetworks {
			existingCIDRs = append(existingCIDRs, subnetwork.IpCidrRange)
		}
	}
	Logger.Sugar().Debugf("[%v] existingCIDRs: %v", vpcName, existingCIDRs)
	// Loop through possible /16 CIDR blocks to find an available block
	for i := 101; i <= 255; i++ {
		// for j := 0; j <= 255; j++ {
		// cidr := fmt.Sprintf("10.%d.%d.0/16", i, j)
		cidr := fmt.Sprintf("10.%d.0.0/16", i)
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			Logger.Error(err.Error())
			return "", err
		}
		conflict := false
		for _, existingCIDR := range existingCIDRs {
			_, existingNet, err := net.ParseCIDR(existingCIDR)
			if err != nil {
				Logger.Error(err.Error())
				return "", err
			}
			if ipnet.Contains(existingNet.IP) || existingNet.Contains(ipnet.IP) {
				conflict = true
				break
			}
		}
		if !conflict {
			Logger.Sugar().Debugf("[%v] found: %v", vpcName, cidr)
			return cidr, nil
		}
		// }
	}
	return "", errors.New("can't find it, VPC's full?")

}
