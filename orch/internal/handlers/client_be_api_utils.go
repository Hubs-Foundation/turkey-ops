package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"main/internal"

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

// type dashboard_hubs struct {
// 	hub_id     pgtype.Int8
// 	account_id pgtype.Int8
// 	name       pgtype.Text
// 	tier       pgtype.Text
// 	subdomain  pgtype.Text
// 	status     pgtype.Text
// }

type turkeyorch_hubs struct {
	fxa_sub pgtype.Text

	hub_id     pgtype.Int8
	account_id pgtype.Int8
	name       pgtype.Text
	tier       pgtype.Text
	subdomain  pgtype.Text
	status     pgtype.Text

	email       pgtype.Text
	inserted_at pgtype.Timestamptz

	domain pgtype.Text
	region pgtype.Text
}
