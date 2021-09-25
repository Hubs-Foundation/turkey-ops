package internal

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

// type Pg struct {
// 	pool *pgxpool.Pool
// }

var PgxPool *pgxpool.Pool

func MakePgxPool() {
	p, err := pgxpool.Connect(context.Background(), cfg.DBconn)
	if err != nil {
		logger.Error("Unable to connect to database: " + err.Error())
	}
	PgxPool = p
}
