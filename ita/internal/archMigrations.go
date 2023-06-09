package internal

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ArchMigrations() error {

	//fixes for pausing
	//		delete junk svcs
	svcs, err := cfg.K8sClientSet.CoreV1().Services(cfg.PodNS).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return err
	}
	for _, svc := range svcs.Items {
		if svc.Name == "ita" {
			continue
		}
		err := cfg.K8sClientSet.CoreV1().Services(cfg.PodNS).Delete(context.Background(), svc.Name, v1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	//		fix pausing label
	pausingLable, err := Deployment_getLabel("pausing")
	if err != nil {
		return err
	}
	if pausingLable == "yes" {
		t_str := time.Now().Format("060102")
		Deployment_setLabel("pausing", t_str) //time.Parse("060102", t_str)

	}

	return nil
}
