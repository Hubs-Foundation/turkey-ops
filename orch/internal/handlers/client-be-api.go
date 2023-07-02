package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"main/internal"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
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

		turkeydashboardPool, _ := pgxpool.Connect(context.Background(), internal.Cfg.DBconn+"/dashboard")

		rows, err := turkeydashboardPool.Query(context.Background(), "SELECT * FROM hubs")
		if err != nil {
			internal.Logger.Sugar().Errorf("Query failed: %v\n", err)
			return
		}
		defer rows.Close()

		hub := dashboard_hubs
		for rows.Next() {
			// Process each row - for example, let's say it's a table of a "name" column
			if err := rows.Scan(&hub); err != nil {
				internal.Logger.Sugar().Errorf("Error scanning row: %v\n", err)
				return
			}
			internal.Logger.Sugar().Debugf("hub: %+v\n", hub)
		}

	}

	http.Error(w, "", 404)
})
