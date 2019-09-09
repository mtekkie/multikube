package clientconfig

import (
	"fmt"
	api2 "github.com/amimof/multikube/pkg/api"
	"k8s.io/client-go/tools/clientcmd/api"
)

type XX struct {
	Hello string
}

type ConfigService struct {

	KubeConfig *api.Config
	CertificateAuthorityData []byte
	ExternalHost string

}

func (c *ConfigService) Read(path string) (interface{}, error) {

	cc := &api.Config{
		Clusters:  make(map[string]*api.Cluster),
		AuthInfos: make(map[string]*api.AuthInfo),
		Contexts:  make(map[string]*api.Context),
	}

	for name, cluster := range c.KubeConfig.Clusters {

		c1 := &api.Cluster{
			Server:                   fmt.Sprintf("https://%s/%s", c.ExternalHost, name),
			CertificateAuthorityData: c.CertificateAuthorityData,
			Extensions:				  cluster.Extensions,
		}
		cc.Clusters[name] = c1

		cctx := &api.Context{
			Cluster:  name,
			AuthInfo: "mk-user",
		}
		cc.Contexts[name] = cctx
	}

	cui := &api.AuthInfo{
		Token: "tobedisplayed",
	}
	cc.AuthInfos["mk-user"] = cui

	return cc, nil
}


func (c *ConfigService) Create (b []byte) (interface{}, error) {
	return nil, api2.ErrNotImplemented


}
