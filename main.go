package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
		Mode string `mapstructure:"mode"`
	} `mapstructure:"server"`

	App struct {
		Name        string `mapstructure:"name"`
		Version     string `mapstructure:"version"`
		Environment string `mapstructure:"environment"`
		Debug       bool   `mapstructure:"debug"`
	} `mapstructure:"app"`

	Azure struct {
		StorageAccount   string `mapstructure:"storage_account"`
		ContainerName    string `mapstructure:"container_name"`
		ConnectionString string `mapstructure:"connection_string"`
	} `mapstructure:"azure"`
	
	Logging struct {
		Level  string `mapstructure:"level"`
		Format string `mapstructure:"format"`
	} `mapstructure:"logging"`
}

var config Config

func loadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	
	viper.AutomaticEnv()
	
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error leyendo archivo de configuración: %v", err)
	}
	
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Error mapeando configuración: %v", err)
	}
	
	log.Printf("Configuración cargada: %s v%s", config.App.Name, config.App.Version)
}

func main() {
	loadConfig()
	
	gin.SetMode(config.Server.Mode)
	
	r := gin.Default()
	
	v1 := r.Group("/api/v1")
	{
		v1.GET("/hello", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message":     "Ok",
				"app_name":    config.App.Name,
				"version":     config.App.Version,
				"environment": config.App.Environment,
			})
		})
		
		if config.App.Environment == "development" {
			v1.GET("/config", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"server": gin.H{
						"host": config.Server.Host,
						"port": config.Server.Port,
						"mode": config.Server.Mode,
					},
					"app": gin.H{
						"name":        config.App.Name,
						"version":     config.App.Version,
						"environment": config.App.Environment,
						"debug":       config.App.Debug,
					},
				})
			})
		}
	}
	
	address := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	log.Printf("Servidor iniciado en %s", address)
	r.Run(address)
}
