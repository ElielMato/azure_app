package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
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
		ConnectionString string `mapstructure:"connection_string"`
	} `mapstructure:"azure"`
	
	Logging struct {
		Level  string `mapstructure:"level"`
		Format string `mapstructure:"format"`
	} `mapstructure:"logging"`
}

var config Config
var telemetryClient appinsights.TelemetryClient

// parseInstrumentationKey extrae la InstrumentationKey del connection string
func parseInstrumentationKey(connectionString string) string {
	parts := strings.Split(connectionString, ";")
	for _, part := range parts {
		if strings.HasPrefix(part, "InstrumentationKey=") {
			return strings.TrimPrefix(part, "InstrumentationKey=")
		}
	}
	return ""
}

// initTelemetry configura Application Insights
func initTelemetry() {
	if config.Azure.ConnectionString == "" {
		log.Println("Warning: Azure connection string no configurado, telemetría no se inicializará")
		return
	}

	instrumentationKey := parseInstrumentationKey(config.Azure.ConnectionString)
	if instrumentationKey == "" {
		log.Println("Warning: No se pudo extraer InstrumentationKey del connection string")
		return
	}

	telemetryConfig := appinsights.NewTelemetryConfiguration(instrumentationKey)
	
	telemetryConfig.EndpointUrl = "https://dc.applicationinsights.azure.com/v2/track"
	
	telemetryClient = appinsights.NewTelemetryClientFromConfig(telemetryConfig)
	
	log.Printf("Application Insights configurado con InstrumentationKey: %s", instrumentationKey[:8]+"...")
}

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
	
	// Inicializar telemetría de Application Insights
	initTelemetry()
	
	gin.SetMode(config.Server.Mode)
	
	r := gin.Default()
	
	// Middleware personalizado para Application Insights
	r.Use(func(c *gin.Context) {
		start := time.Now()
		
		c.Next()
		
		// Enviar telemetría de request si está configurado
		if telemetryClient != nil {
			duration := time.Since(start)
			
			request := appinsights.NewRequestTelemetry(
				c.Request.Method,
				c.Request.URL.String(),
				duration,
				fmt.Sprintf("%d", c.Writer.Status()),
			)
			
			request.Properties["route"] = c.FullPath()
			request.Properties["user_agent"] = c.Request.UserAgent()
			request.Properties["app_version"] = config.App.Version
			
			telemetryClient.Track(request)
		}
	})
	
	v1 := r.Group("/api/v1")
	{
		v1.GET("/hello", func(c *gin.Context) {
			start := time.Now()
			
			// Enviar telemetría personalizada
			if telemetryClient != nil {
				event := appinsights.NewEventTelemetry("hello_endpoint_called")
				event.Properties["app_name"] = config.App.Name
				event.Properties["version"] = config.App.Version
				event.Properties["environment"] = config.App.Environment
				telemetryClient.Track(event)
			}
			
			// Simular algún trabajo
			time.Sleep(10 * time.Millisecond)
			
			response := gin.H{
				"message":     "Ok",
				"app_name":    config.App.Name,
				"version":     config.App.Version,
				"environment": config.App.Environment,
				"timestamp":   time.Now().Unix(),
			}
			
			c.JSON(200, response)
			
			// Enviar métrica personalizada
			if telemetryClient != nil {
				duration := time.Since(start)
				metric := appinsights.NewMetricTelemetry("hello_response_time", duration.Seconds())
				metric.Properties["endpoint"] = "/hello"
				telemetryClient.Track(metric)
			}
		})
		
		if config.App.Environment == "development" {
			v1.GET("/config", func(c *gin.Context) {
				if telemetryClient != nil {
					event := appinsights.NewEventTelemetry("config_endpoint_accessed")
					event.Properties["debug_mode"] = "true"
					telemetryClient.Track(event)
				}
				
				response := gin.H{
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
					"telemetry": gin.H{
						"enabled":             config.Azure.ConnectionString != "",
						"instrumentation_key": parseInstrumentationKey(config.Azure.ConnectionString),
					},
				}
				
				c.JSON(200, response)
			})
		}
	}
	
	address := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	log.Printf("Servidor iniciado en %s con Application Insights habilitado", address)
	r.Run(address)
}
