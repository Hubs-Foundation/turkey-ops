package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"main/internal"
	mrand "math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/form3tech-oss/jwt-go"
	"github.com/jackc/pgtype"
	"github.com/tanfarming/goutils/pkg/kubelocker"
)

// func dashboardDb_get_turkeyAccountId(fxaSub string) pgx.Rows {
// 	rows, _ := internal.DashboardDb.Query(context.Background(),
// 		fmt.Sprintf(`select account_id from accounts where fxa_uid=%v`, fxaSub))
// 	return rows
// }

// func dashboardDb_get_Hubs(turkeyAccountId string) pgx.Rows {
// 	rows, _ := internal.DashboardDb.Query(context.Background(),
// 		fmt.Sprintf(`select * from accounts where account_id=%v`, turkeyAccountId))
// 	return rows
// }

// func dashboardDb_get_hubs_for_fxaSub(fxaSub string) pgx.Rows {
// 	rows, _ := internal.DashboardDb.Query(context.Background(),
// 		fmt.Sprintf(`SELECT h.* FROM hubs h INNER JOIN accounts a ON h.account_id = a.account_id WHERE a.fxa_uid = '%v'`, fxaSub))
// 	return rows
// }

func DashboardDb_getHubs(t0 time.Time) (map[int64]Turkeyorch_hubs, error) {
	internal.Logger.Sugar().Debugf("getting * since: %v", t0)
	hubs := make(map[int64]Turkeyorch_hubs)
	rows, err := internal.DashboardDb.Query(context.Background(), "SELECT hub_id, name, tier, subdomain, status, account_id FROM hubs where inserted_at>$1", t0)
	if err != nil {
		internal.Logger.Sugar().Errorf("Query failed: %v", err)
		return hubs, err
	}
	defer rows.Close()
	_hub := Turkeyorch_hubs{}
	for rows.Next() {

		if err := rows.Scan(&_hub.Hub_id, &_hub.Name, &_hub.Tier, &_hub.Subdomain, &_hub.Status, &_hub.Account_id); err != nil {
			internal.Logger.Sugar().Errorf("Error scanning row: %v", err)
			return hubs, err
		}
		DashboardDb_fattenHub(&_hub)

		hubs[_hub.Hub_id.Int] = _hub
		// internal.Logger.Sugar().Debugf("hub: %+v\n", _hub)
	}
	return hubs, err
}

func DashboardDb_fattenHub(_hub *Turkeyorch_hubs) error {

	internal.DashboardDb.QueryRow(context.Background(),
		`select fxa_uid, email, inserted_at from accounts where account_id=($1)`, _hub.Account_id.Int).
		Scan(&_hub.Fxa_sub, &_hub.Email, &_hub.Inserted_at)

	internal.DashboardDb.QueryRow(context.Background(),
		`select domain from hub_deployments where hub_id=($1)`, _hub.Hub_id.Int).
		Scan(&_hub.Domain)

	_hub.Region.String = "us"

	return nil
}

func OrchDb_insertHub(hub Turkeyorch_hubs) error {

	if hub.Email.String == "" {
		internal.Logger.Sugar().Warnf("bad (empty email), drop: %+v", hub)
		return nil
	}
	_, err := internal.OrchDb.Exec(
		context.Background(),
		`insert into hubs (hub_id,account_id,fxa_sub,name,tier,subdomain,status,email,domain,region,inserted_at) values ($1, $2, $3, $4, $5, $6, $7, $8, $9,$10,$11)`,
		hub.Hub_id.Int, hub.Account_id.Int, hub.Fxa_sub.String, hub.Name.String, hub.Tier.String, hub.Subdomain.String, hub.Status.String, hub.Email.String, hub.Domain.String, hub.Region.String, hub.Inserted_at.Time)
	if err == nil {
		internal.Logger.Sugar().Debugf("loaded: %+v", hub)
	}
	return err
}
func OrchDb_insertHubs(hubs map[int64]Turkeyorch_hubs) {
	internal.Logger.Sugar().Debugf("upserting <%v> hubs", len(hubs))
	for _, hub := range hubs {
		err := OrchDb_insertHub(hub)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to upsert: <%+v>, err: %+v", hub, err)
		}
	}
}
func OrchDb_upsertHub(hub Turkeyorch_hubs) error {
	sql := `
		INSERT INTO hubs (hub_id, account_id, fxa_sub, name, tier, status,email, subdomain, inserted_at, domain, region) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) 
		ON CONFLICT (hub_id) 
		DO UPDATE SET account_id=$2,fxa_sub=$3,name=$4,tier=$5,status=$6,email=$7,subdomain=$8,inserted_at=$9,domain=$10,region=$11
		WHERE hubs.hub_id = $1;
	`
	_, err := internal.OrchDb.Exec(context.Background(),
		sql,
		hub.Hub_id.Int, hub.Account_id.Int, hub.Fxa_sub.String, hub.Name.String, hub.Tier.String, hub.Status.String, hub.Email.String, hub.Subdomain.String,
		hub.Inserted_at.Time, hub.Domain.String, hub.Region.String)
	return err
}
func OrchDb_upsertHubs(hubs map[int64]Turkeyorch_hubs) {
	internal.Logger.Sugar().Debugf("upserting <%v> hubs", len(hubs))
	for _, hub := range hubs {
		err := OrchDb_upsertHub(hub)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to upsert: <%+v>, err: %+v", hub, err)
		}
	}
}

func OrchDb_upsertAcct(hub Turkeyorch_hubs) error {
	sql := `
		INSERT INTO accounts (account_id, fxa_sub, email, inserted_at) 
		VALUES ($1, $2, $3, $4) 
		ON CONFLICT (account_id) 
		DO UPDATE SET account_id=$1,fxa_sub=$2,email=$3,inserted_at=$4
		WHERE accounts.account_id = $1;
	`
	_, err := internal.OrchDb.Exec(context.Background(),
		sql,
		hub.Account_id.Int, hub.Fxa_sub.String, hub.Email.String, hub.Inserted_at.Time)
	return err
}

func OrchDb_upsertAccts(hubs map[int64]Turkeyorch_hubs) {
	internal.Logger.Sugar().Debugf("upserting <%v> hubs", len(hubs))
	for _, hub := range hubs {
		err := OrchDb_upsertAcct(hub)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to upsert: <%+v>, err: %+v", hub, err)
		}
	}
}

func OrchDb_getHub(hubId string) Turkeyorch_hubs {
	hub := Turkeyorch_hubs{}
	internal.OrchDb.QueryRow(context.Background(),
		"select account_id,fxa_sub,name,tier,status,email,subdomain,inserted_at,domain,region from hubs where hub_id=$1", hubId).Scan(&hub)
	return hub
}

func OrchDb_updateHub_status(hubId, status string) error {
	_, err := internal.OrchDb.Exec(context.Background(),
		"update hubs set status=$2 where hub_id=$1",
		hubId, status)
	return err
}

func OrchDb_updateHub_tier(hubId, tier string) error {
	_, err := internal.OrchDb.Exec(context.Background(),
		"update hubs set tier=$2 where hub_id=$1",
		hubId, tier)
	return err
}

func OrchDb_updateHub_subdomain(hubId, subdomain string) error {
	_, err := internal.OrchDb.Exec(context.Background(),
		"update hubs set subdomain=$2 where hub_id=$1",
		hubId, subdomain)
	return err
}

func OrchDb_deleteHub(hubId string) error {
	_, err := internal.OrchDb.Exec(context.Background(),
		"delete from hubs where hub_id=$1",
		hubId)
	return err
}

// User is the authenticated user
type fxaUser struct {
	Exp                  int64    `json:"exp"`
	TwoFA                bool     `json:"fxa_2fa"`
	Cancel_at_period_end bool     `json:"fxa_cancel_at_period_end"`
	Current_period_end   float64  `json:"fxa_current_period_end"`
	DisplayName          string   `json:"fxa_displayName"`
	Email                string   `json:"fxa_email"`
	Avatar               string   `json:"fxa_pic"`
	Plan_id              string   `json:"fxa_plan_id"`
	Subscriptions        []string `json:"fxa_subscriptions"`
	Iat                  int64    `json:"iat"`
	Iss                  string   `json:"iss"`
	Sub                  string   `json:"sub"`
}

func CheckAndReadJwtToken(jwtToken string) (fxaUser, error) {
	var fxaUser fxaUser

	token, err := jwt.Parse(
		jwtToken,
		func(token *jwt.Token) (interface{}, error) {
			return internal.Cfg.PermsKey_pub, nil
		})
	if err != nil {
		return fxaUser, err
	}
	internal.Logger.Sugar().Debugf("token: %v", token)
	if !token.Valid {
		return fxaUser, errors.New("invalid token")
	}
	claimMap := token.Claims.(jwt.MapClaims)
	claimBytes, err := json.Marshal(claimMap)
	if err != nil {
		return fxaUser, fmt.Errorf("error marshaling claimMap: %v", err)
	}
	err = json.Unmarshal(claimBytes, &fxaUser)
	if err != nil {
		return fxaUser, fmt.Errorf("error unmarshaling claimBytes to fxaUser: %v", err)
	}

	return fxaUser, nil
}

type Turkeyorch_hubs struct {
	Fxa_sub pgtype.Text

	Hub_id     pgtype.Int8
	Account_id pgtype.Int8
	Name       pgtype.Text
	Tier       pgtype.Text
	Subdomain  pgtype.Text
	Status     pgtype.Text

	Email       pgtype.Text
	Inserted_at pgtype.Timestamptz

	Domain pgtype.Text
	Region pgtype.Text
}

func Cronjob_syncDashboardDb(interval time.Duration) {

	locker, err := kubelocker.NewNamed(internal.Cfg.K8ss_local.ClientSet, internal.Cfg.PodNS, "sync_dashboard_db")
	if err != nil {
		internal.Logger.Sugar().Errorf("failed to create locker for sync_dashboard_db: %v", err)
	} else {
		err = locker.Lock()
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to lock: err:%v, id: %v, worklog: %v", err, locker.Id(), strings.Join(locker.WorkLog(), ";"))
		}
		internal.Logger.Sugar().Debugf("acquired locker: %v \n", locker.Id())
		defer func() {
			err = locker.Unlock()
			if err != nil {
				internal.Logger.Sugar().Errorf("failed to unlock " + err.Error())
			}
		}()
	}

	lastSyncStr, err := internal.Cfg.Redis.Get("syncDashboardDb")
	if err == nil && lastSyncStr != "" {
		lastSync, err := time.Parse(time.RFC3339, lastSyncStr)
		if err == nil && time.Since(lastSync) < 250*time.Second {
			internal.Logger.Sugar().Debugf("skip -- time.Since(lastSync): %v", time.Since(lastSync))
			return
		}
	}

	t0 := time.Now()

	var orchT pgtype.Timestamptz
	internal.OrchDb.QueryRow(context.Background(), "select inserted_at from hubs order by inserted_at desc limit 1;").Scan(&orchT)

	hubs, err := DashboardDb_getHubs(orchT.Time)
	if err != nil {
		internal.Logger.Sugar().Errorf("failed: %v", err)
		return
	}
	OrchDb_upsertHubs(hubs)
	OrchDb_upsertAccts(hubs)

	internal.Logger.Sugar().Debugf("synced (%v) hubs, took: %v", len(hubs), time.Since(t0))

	internal.Cfg.Redis.Set("syncDashboardDb", time.Now().Format(time.RFC3339))

}

func UpdateOrchDb(task string, cfg HCcfg) {

	//update orchDb
	switch task {
	case "hc_create":
		accountId, err := strconv.ParseInt(cfg.AccountId, 10, 64)
		if err != nil {
			internal.Logger.Sugar().Errorf("accountId cannot be parsed into int64: %v", cfg.AccountId)
			return
		}
		hubId, err := strconv.ParseInt(cfg.HubId, 10, 64)
		if err != nil {
			internal.Logger.Sugar().Warnf("failed to convert cfg.HubId(%v)", hubId)
			hubId = time.Now().UnixNano()
			internal.Logger.Sugar().Warnf("using time.Now().UnixNano() (%v)", hubId)
		}
		OrchDb_upsertHub(
			Turkeyorch_hubs{
				Hub_id:      pgtype.Int8{Int: int64(hubId)},
				Account_id:  pgtype.Int8{Int: accountId},
				Fxa_sub:     pgtype.Text{String: cfg.FxaSub},
				Name:        pgtype.Text{String: cfg.Name},
				Tier:        pgtype.Text{String: cfg.Tier},
				Status:      pgtype.Text{String: "ready"},
				Email:       pgtype.Text{String: cfg.UserEmail},
				Subdomain:   pgtype.Text{String: cfg.Subdomain},
				Inserted_at: pgtype.Timestamptz{Time: time.Now()},
				Domain:      pgtype.Text{String: cfg.Domain},
				Region:      pgtype.Text{String: cfg.Region},
			})
	case "hc_delete":
		OrchDb_deleteHub(cfg.HubId)
	case "hc_switch_up":
		OrchDb_updateHub_status(cfg.HubId, "up")
	case "hc_switch_down":
		OrchDb_updateHub_status(cfg.HubId, "down")
	case "hc_collect":
		OrchDb_updateHub_status(cfg.HubId, "collected")
	case "hc_restore":
		OrchDb_updateHub_status(cfg.HubId, "ready")
	case "hc_update":
		if cfg.Tier != "" && cfg.CcuLimit != "" && cfg.StorageLimit != "" {
			OrchDb_updateHub_tier(cfg.HubId, cfg.Tier)
		}
		if cfg.Subdomain != "" {
			OrchDb_updateHub_subdomain(cfg.HubId, cfg.Subdomain)
		}
	}
}

func AccountIdGen() int64 {

	// internal.Cfg.PodIp
	return time.Now().UnixNano() - 1690000000000000000 + mrand.Int63n(100)
}
