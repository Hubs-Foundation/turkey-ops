package internal

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

var PgxPool *pgxpool.Pool

func MakePgxPool() {
	p, err := pgxpool.Connect(context.Background(), Cfg.DBconn)
	if err != nil {
		logger.Error("Unable to connect to database: " + err.Error())
	}
	logger.Debug(fmt.Sprintf("pgxpool.Pool.Stat: %v\n", p.Stat()))
	PgxPool = p
}
