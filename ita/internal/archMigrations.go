package internal

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ArchMigrations() error {

	Logger.Debug("placeholder")

	pods, err := cfg.K8sClientSet.CoreV1().Pods(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error("ArchMigrations -- failed to list pods: " + err.Error())
		return err
	}
	for _, pod := range pods.Items {
		Logger.Info("pod.Name: " + pod.Name)
		if !strings.HasPrefix(pod.Name, "ita-") {
			return nil
		}
		orchCollect()
	}
	return nil
}
