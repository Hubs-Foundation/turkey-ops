package main

import (
	"main/internal"
	"main/internal/handlers"
	"net/http"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	turkeyDomain string
)

func main() {
	turkeyDomain = "myhubs.net"

	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = ""
	logger := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.Lock(os.Stdout), atom))
	atom.SetLevel(zap.DebugLevel)

	internal.MakeCfg(logger)

	router := http.NewServeMux()

	router.Handle("/healthz", handlers.Healthz())

	router.Handle("/login", handlers.Login())
	router.Handle("/logout", handlers.Logout())

	router.Handle("/traefik-ip", handlers.TraefikIp())
	router.Handle("/_oauth", handlers.Oauth())

	internal.StartServer(router, 9001)

}
