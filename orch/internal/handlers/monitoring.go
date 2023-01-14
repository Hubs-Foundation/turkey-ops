package handlers

import (
	"io/ioutil"
	"main/internal"
	"os"
)

type MonitoringCfg struct {
	Env string
}

func DeployMonitoring() {
	env := os.Getenv("ENV")

	mCfg := MonitoringCfg{env}

	if env == "" {
		internal.Logger.Error("could not get ENV when deploying monitoring")
		return
	}

	yamBytes, err := ioutil.ReadFile("./_files/yams/addons/monitoring.yam")
	if err != nil {
		internal.Logger.Error("failed to get monitoring yam file because: " + err.Error())
		return
	}

	renderedYamls, err := internal.K8s_render_yams([]string{string(yamBytes)}, mCfg)
	if err != nil {
		internal.Logger.Error("failed to render monitoring yam file because: " + err.Error())
		return
	}

	k8sChartYaml := renderedYamls[0]

	internal.Logger.Debug("&#128640; --- start deploying monitoring")
	// deploy yamls
	err = internal.Ssa_k8sChartYaml("turkey_cluster", k8sChartYaml, internal.Cfg.K8ss_local.Cfg) // ServerSideApply version of kubectl apply -f
	if err != nil {
		internal.Logger.Error("monitoring deployment: failed @ Ssa_k8sChartYaml" + err.Error())
		return
	}
}
