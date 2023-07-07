package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"main/internal"
	"net/http"
	"strings"
)

// wip

var DashboardApi = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		http.Error(w, "", 404)
		return
	}
	tokenCookie, err := r.Cookie("_turkeyauthtoken")
	if err != nil {
		http.Error(w, "", 404)
		return
	}
	fxaUser, err := CheckAndReadJwtToken(tokenCookie.Value)
	if err != nil {
		if err.Error() != "invalid token" {
			internal.Logger.Sugar().Debugf("CheckAndReadJwtToken err: %v", err)
		}
		http.Error(w, "", 404)
		return
	}

	reqId := w.Header().Get("X-Request-Id")
	internal.Logger.Sugar().Debugf("[%v] fxaUser: %v", reqId, fxaUser)

	// rows := db_get_hubs_for_fxaSub(fxaUser.Sub)

	resource := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	internal.Logger.Sugar().Debugf("resource: %v", resource)

	switch resource {
	case "account":

		json.NewEncoder(w).Encode(
			[]map[string]interface{}{
				{
					"displayName":     fxaUser.DisplayName,
					"email":           fxaUser.Email,
					"hasCreatingHubs": false,                          //
					"hasHubs":         true,                           //
					"hasPlan":         fxaUser.Plan_id != "",          //
					"hasSubscription": len(fxaUser.Subscriptions) > 0, //
					"isForbidden":     false,                          //
					"planName":        "standard",
					"profilePic":      fxaUser.Avatar,
				},
			},
		)
	case "plans":
		fmt.Fprintf(w, "not yet")
	case "subscription":
		fmt.Fprintf(w, "not yet")
	case "hubs":

		json.NewEncoder(w).Encode(
			[]map[string]interface{}{
				{
					"ccuLimit":         25,
					"currentCcu":       0,
					"currentStorageMb": 72.1640625,
					"domain":           "null",
					"hubId":            "268704415735611520",
					"name":             "Untitled Hub",
					"region":           "null",
					"status":           "ready",
					"storageLimitMb":   2000,
					"subdomain":        "gtan-moz",
					"tier":             "p1",
				},
			},
		)
	case "events/fxa":
		fmt.Fprintf(w, "not yet")

	case "z/load_from_dashboard":
		// fmt.Fprintf(w, "z/load_from_dashboard: %+v\n", fxaUser)

		// turkeydashboardPool, _ := pgxpool.Connect(context.Background(), internal.Cfg.DBconn+"/dashboard")

		hubs := make(map[int64]turkeyorch_hubs)

		rows, err := internal.DashboardDb.Query(context.Background(), "SELECT hub_id, name, tier, subdomain, status, account_id FROM hubs")
		if err != nil {
			internal.Logger.Sugar().Errorf("Query failed: %v", err)
			return
		}
		defer rows.Close()
		_hub := turkeyorch_hubs{}
		for rows.Next() {

			if err := rows.Scan(&_hub.hub_id, &_hub.name, &_hub.tier, &_hub.subdomain, &_hub.status, &_hub.account_id); err != nil {
				internal.Logger.Sugar().Errorf("Error scanning row: %v", err)
				return
			}

			acct_row := internal.DashboardDb.QueryRow(context.Background(), `select fxa_uid, email, inserted_at from accounts where account_id=`+_hub.account_id.String)

			acct_row.Scan(&_hub.fxa_sub, &_hub.email, &_hub.inserted_at)

			hubs[_hub.hub_id.Int] = _hub

			// internal.Logger.Sugar().Debugf("hub: %+v\n", _hub)
		}
		// internal.Logger.Sugar().Debugf("hubs: %+v\n", hubs)
		for _, v := range hubs {
			_, err := internal.OrchDb.Exec(
				context.Background(),
				`insert into hubs (hub_id,account_id,fxa_sub,name,tier,subdomain,status,email,inserted_at) values ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				v.hub_id, v.account_id, v.fxa_sub, v.name, v.tier, v.subdomain, v.status, v.email, v.inserted_at,
			)
			if err != nil {
				internal.Logger.Sugar().Errorf("failed to insert: %v", err)
			}

		}

	}

	http.Error(w, "", 404)
})
