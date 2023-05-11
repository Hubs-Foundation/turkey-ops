package internal

import (
	"os"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/strings/slices"
)

///////////////////////////////////////////////////////////

type featureMan struct {
	_features hubFeatures
	mu        sync.Mutex
}

type hubFeatures struct {
	updater      bool
	customDomain bool
	customClient bool
}

func New_featureMan() featureMan {
	return featureMan{
		_features: hubFeatures{
			updater:      false,
			customDomain: false,
			customClient: false,
		},
	}
}

func (fm *featureMan) Get() hubFeatures {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	return fm._features
}

func (fm *featureMan) determineFeatures() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if slices.Contains([]string{"dev"}, cfg.Tier) {
		fm._features.updater = true
		fm._features.customDomain = true
		fm._features.customClient = true
		return
	}

	if _, noUpdates := os.LookupEnv("NO_UPDATES"); !noUpdates {
		fm._features.updater = true
	}

	if slices.Contains([]string{"pro", "business", "b1"}, cfg.Tier) {
		fm._features.customDomain = true

		customDomain, _ := Deployment_getLabel("custom-domain")
		if customDomain != "" {
			Logger.Info("customClient enabled -- customDomain: " + customDomain)
			fm._features.customClient = true
		}
	}

}

func (fm *featureMan) enableCustomClient() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm._features.customClient = true
}

func (fm *featureMan) setupFeatures() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	Logger.Sugar().Infof("initFeatures -- cfg.Features: %+v", fm._features)

	if fm._features.updater {
		cfg.TurkeyUpdater = NewTurkeyUpdater()
		err := cfg.TurkeyUpdater.Start()
		if err != nil {
			Logger.Error(err.Error())
		}
	}

	if fm._features.customDomain {
		err := ingress_addItaApiRule()
		if err != nil {
			Logger.Error(err.Error())
		}
	}

	if fm._features.customClient {
		err := k8s_mountRetNfs("ita", "", "", false, corev1.MountPropagationNone)
		if err != nil {
			Logger.Error(err.Error())
		}
		blockEgress("hubs")
		blockEgress("spoke")
	}
}

/////////////////////////////////////////////////////////////
