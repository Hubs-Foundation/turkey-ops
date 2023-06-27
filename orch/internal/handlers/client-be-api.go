package handlers

import (
	"encoding/json"
	"fmt"
	"main/internal"
	"net/http"
	"strings"
)

// wip

var Api_v1 = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		http.Error(w, "", 404)
		return
	}
	token, err := r.Cookie("_turkeyauthtoken")
	if err != nil {
		http.Error(w, "", 404)
		return
	}
	fxaUser, err := CheckAndReadJwtToken(token.Value)
	if err != nil {
		http.Error(w, "", 404)
		return
	}

	reqId := w.Header().Get("X-Request-Id")
	internal.Logger.Sugar().Debugf("[%v] fxaUser: %v", reqId, fxaUser)

	// rows := db_get_hubs_for_fxaSub(fxaUser.Sub)

	resource := strings.TrimPrefix(r.URL.Path, "/api/v1/")
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

	}

	// hubId := "11111111111111"
	// hubRec, _ := internal.PgxPool.Exec(context.Background(),
	// 	fmt.Sprintf(`select * from hubs where datname = 'dashboard' and hub_id=%v`, hubId))

})
