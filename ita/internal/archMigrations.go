package internal

import (
	"context"
	"strings"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ArchMigrations() error {

	pausingLable, err := Deployment_getLabel("pausing")
	if err != nil {
		return err
	}
	// if pausingLable != "yes" {
	// 	return nil
	// }

	//double check
	ret_d, _ := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), "ret", v1.GetOptions{})
	if *ret_d.Spec.Replicas != 0 {
		Logger.Error("VERY BAD -- unexpceted paused instance, pausing==yes, but ret.replicas != 0, manual investigation needed")
		return nil
	}
	//tripple check
	pods, _ := cfg.K8sClientSet.CoreV1().Pods(cfg.PodNS).List(context.Background(), v1.ListOptions{})
	if len(pods.Items) != 1 {
		return nil
	}
	if !strings.HasPrefix(pods.Items[0].Name, "ita-") {
		return nil
	}

	//fixes for pausing
	//		delete junk svcs
	svcs, err := cfg.K8sClientSet.CoreV1().Services(cfg.PodNS).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return err
	}
	for _, svc := range svcs.Items {
		if svc.Name != "ita" {
			err := cfg.K8sClientSet.CoreV1().Services(cfg.PodNS).Delete(context.Background(), svc.Name, v1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}
	//		fix pausing label
	if pausingLable == "yes" {
		t_str := time.Now().Format("060102")
		Deployment_setLabel("pausing", t_str) //time.Parse("060102", t_str)

	}

	return nil
}
