package handlers

import (
	"context"
	"encoding/json"
	"main/internal"

	"cloud.google.com/go/pubsub"
)

func HandleTurkeyJobs() {

	internal.Logger.Debug("")

	go func() {
		for {
			err := internal.Cfg.Gcps.PubSub_Pulling(
				internal.Cfg.TurkeyJobsPubSubSubName,
				TurkeyJobRouter,
			)
			internal.Logger.Sugar().Errorf("err: %v", err)
		}
	}()
}

var TurkeyJobRouter = func(_ context.Context, msg *pubsub.Message) {
	internal.Logger.Sugar().Debugf("received message, msg.Data :%v\n", string(msg.Data))
	internal.Logger.Sugar().Debugf("received message, msg.Attributes :%v\n", msg.Attributes)
	internal.Logger.Sugar().Debugf("HC_Count: %v", internal.HC_Count)

	var hcCfg HCcfg
	err := json.Unmarshal(msg.Data, &hcCfg)
	if err != nil {
		internal.Logger.Error("bad hcmsg.DataCfg: " + string(msg.Data))
	}

	internal.Logger.Sugar().Debugf("hcCfg: %v", hcCfg)

	switch hcCfg.TurkeyJobReqMethod {
	case "POST":
		err = CreateHubsCloudInstance(hcCfg)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to CreateHubsCloudInstance, err: %v, msg.Data dump: %v", err, string(msg.Data))
		}
	case "PATCH":
		_, err := UpdateHubsCloudInstance(hcCfg)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to PatchHubsCloudInstance, err: %v, msg.Data dump: %v", err, string(msg.Data))
		}
	case "DELETE":
		err := DeleteHubsCloudInstance(hcCfg)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to DeleteHubsCloudInstance, err: %v, msg.Data dump: %v", err, string(msg.Data))
		}

	default:
		internal.Logger.Error("bad hcCfg.TurkeyJobReqMethod: " + hcCfg.TurkeyJobReqMethod)
	}
	msg.Ack()
}
