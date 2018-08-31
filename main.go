package main

import (
	"os"

	"github.com/gin-gonic/gin"
	logging "github.com/op/go-logging"
	"gitlab.luojilab.com/luojilabpythoner/entropy-core/config"
)

var router *gin.Engine

func main() {
	logFile := initLogConf()
	defer logFile.Close()
	// init routes
	router = gin.Default()
	//router.LoadHTMLGlob("templates/*")

	// set Mode
	if config.Conf.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	initializeRoutes()

	router.Run(":8080")
}

func initLogConf() *os.File {
	//init logging
	var formatter = logging.MustStringFormatter(
		`[%{level:.4s}] %{time:2006-01-02 15:04:05.000} %{shortfile} %{message}`,
	)
	var logPath string
	logPath = "log.txt"
	if config.Conf.Debug {
		logPath = "service-log.txt"
	} else {
		logPath = "/data/logs/entropy-service.log"
	}
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	backend := logging.NewLogBackend(logFile, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, formatter)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(logging.INFO, "")

	logging.SetBackend(backendLeveled)
	return logFile
}
