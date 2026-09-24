package main

import (
	"LinkDesk/config"
	"LinkDesk/db"
	"LinkDesk/handler"
	"LinkDesk/middleware"
	"fmt"

	"github.com/gin-gonic/gin"
)

func main() {
	errCfg := config.LoadConfig()
	if errCfg != nil {
		fmt.Println(errCfg)
		return
	}
	errDb := db.Connect()
	if errDb != nil {
		fmt.Println(errDb)
		return
	}

	r := gin.Default()
	r.Use(middleware.CORS())
	handler.RegisterRoutes(r)

	r.Run(config.Config.Server.Port)
}
