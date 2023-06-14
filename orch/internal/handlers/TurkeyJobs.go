package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"main/internal"
	"math"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub"
)

func HandleTurkeyJobs() {

	internal.Logger.Debug("HandleTurkeyJobs")

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

	if internal.Cfg.LAZY {
		internal.Logger.Info("LAZY --> Nack")
		msg.Nack()
		return
	}

	DeliveryAttempt := *msg.DeliveryAttempt
	internal.Logger.Sugar().Debugf("received message, msg.Data :%v\n", string(msg.Data))
	internal.Logger.Sugar().Debugf("received message, msg.DeliveryAttempt :%v\n", DeliveryAttempt)
	internal.Logger.Sugar().Debugf("received message, msg.ID :%v\n", msg.ID)
	internal.Logger.Sugar().Debugf("received message, msg.OrderingKey :%v\n", msg.OrderingKey)
	internal.Logger.Sugar().Debugf("received message, msg.PublishTime :%v\n", msg.PublishTime)
	internal.Logger.Sugar().Debugf("HC_Count: %v", internal.HC_Count)

	//let lighter clusters take the job first
	loadTier := (int)(math.Round((float64)(internal.HC_Count) / (float64)(1000)))

	internal.Logger.Sugar().Debugf("DeliveryAttempt(%v), loadTier(%v)", DeliveryAttempt, loadTier)
	if DeliveryAttempt < 6 && DeliveryAttempt < loadTier {
		internal.Logger.Sugar().Debugf("Nack: DeliveryAttempt(%v), loadTier(%v)", DeliveryAttempt, loadTier)
		msg.Nack()
		return
	}
	snooze := time.Duration(internal.HC_Count * int32(time.Millisecond))
	internal.Logger.Sugar().Debugf("snoozing for HC_Count: %v Millisecond", snooze.Milliseconds())
	time.Sleep(snooze)

	//try to acquire the job
	AckStat, err := msg.AckWithResult().Get(context.Background())
	if err != nil {
		internal.Logger.Sugar().Infof("[abort] AckStat: %v, AckWithResult err: %v,%v", AckStat, err)
		return
	}

	// job acquired
	var hcCfg HCcfg
	err = json.Unmarshal(msg.Data, &hcCfg)
	if err != nil {
		internal.Logger.Error("bad msg.Data: " + string(msg.Data))
	}

	internal.Logger.Sugar().Debugf("hcCfg: %v", hcCfg)
	hcCfg, err = makeHcCfg(hcCfg)
	if err != nil {
		internal.Logger.Error("bad hcCfg: " + err.Error())
		return
	}

	callback_payload := map[string]string{}
	callback_payload["id"] = hcCfg.TurkeyJobJobId
	callback_payload["hub_id"] = hcCfg.HubId
	callback_payload["domain"] = hcCfg.HubDomain

	switch hcCfg.TurkeyJobReqMethod {
	case "POST":
		err = CreateHubsCloudInstance(hcCfg)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to CreateHubsCloudInstance, err: %v", err)
			callback_payload["err"] = err.Error()
		}
	case "PATCH":
		_, err := UpdateHubsCloudInstance(hcCfg)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to PatchHubsCloudInstance, err: %v", err)
			callback_payload["err"] = err.Error()
		}
	case "DELETE":
		err := DeleteHubsCloudInstance(hcCfg)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to DeleteHubsCloudInstance, err: %v", err)
			callback_payload["err"] = err.Error()
		}
	default:
		internal.Logger.Warn("bad hcCfg.TurkeyJobReqMethod: " + hcCfg.TurkeyJobReqMethod)
	}

	//callback
	internal.Logger.Sugar().Debugf("calling back: %v", hcCfg.TurkeyJobCallback)
	jsonPayload, _ := json.Marshal(callback_payload)

	_, err = http.Post(hcCfg.TurkeyJobCallback, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		internal.Logger.Error("callback failed: " + err.Error())
		return
	}

}
