package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"main/internal"
	"time"

	"github.com/form3tech-oss/jwt-go"
	"github.com/jackc/pgtype"
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

func OrchDb_loadHub(hub Turkeyorch_hubs) error {

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
func OrchDb_loadHubs(hubs map[int64]Turkeyorch_hubs) {
	for _, hub := range hubs {
		err := OrchDb_loadHub(hub)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to load: <%+v>", hub)
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
	_, err := internal.OrchDb.Exec(context.Background(), "update hubs set status=$2 where hub_id=$1", status, hubId)
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
	var orchT pgtype.Timestamptz
	internal.OrchDb.QueryRow(context.Background(), "select inserted_at from hubs order by inserted_at desc limit 1;").Scan(&orchT)

	hubs, err := DashboardDb_getHubs(orchT.Time)
	if err != nil {
		internal.Logger.Sugar().Errorf("failed: %v", err)
		return
	}
	OrchDb_loadHubs(hubs)

}
