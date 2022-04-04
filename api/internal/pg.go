package internal

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

var PgxPool *pgxpool.Pool

func MakePgxPool() {
	p, err := pgxpool.Connect(context.Background(), Cfg.DBconn+"/portal")
	if err != nil {
		logger.Error("Unable to connect to database: " + err.Error())
	}
	PgxPool = p
}

// func UpsertHubStatus(hubId, status string) error {
// 	_, err := PgxPool.Exec(
// 		context.Background(),
// 		fmt.Sprintf(`INSERT into hubs (hub_id, status) values ('%v','%v') ON CONFLICT DO UPDATE`, hubId, status),
// 	)
// 	if err != nil {
// 		logger.Error(err.Error())
// 		return err
// 	}
// 	return nil
// }
