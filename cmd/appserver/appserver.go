package main

import (
	"log"
	"goships/internal/appserver/config"
)

func main() {
	config.Init("appserver")

	log.Printf("Server: %+v \n", config.Conf.Server)
}