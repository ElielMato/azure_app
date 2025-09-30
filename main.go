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
	Name        string `mapstructure:"app-name"`
	Version     string `mapstructure:"app-version"`
	ConnectionString string `mapstructure:"azure-connection-string"`
	Level  string `mapstructure:"logging-level"`
	Format string `mapstructure:"logging-format"`
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
	if config.ConnectionString == "" {
		log.Println("Warning: Azure connection string no configurado, telemetría no se inicializará")
		return
	}

	instrumentationKey := parseInstrumentationKey(config.ConnectionString)
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
	
	log.Printf("Configuración cargada: %s v%s", config.Name, config.Version)
}

func main() {
	loadConfig()
	
	// Inicializar telemetría de Application Insights
	initTelemetry()
	
	// gin.SetMode(config.Server.Mode)
	
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
			request.Properties["app_version"] = config.Version
			
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
				event.Properties["app_name"] = config.Name
				event.Properties["version"] = config.Version
				telemetryClient.Track(event)
			}
			
			// Simular algún trabajo
			time.Sleep(10 * time.Millisecond)
			
			response := gin.H{
				"message":     "Ok",
				"app_name":    config.Name,
				"version":     config.Version,
				"timestamp":   time.Now().Unix(),
			}
			
			c.JSON(200, response)
			
			if telemetryClient != nil {
				duration := time.Since(start)
				metric := appinsights.NewMetricTelemetry("hello_response_time", duration.Seconds())
				metric.Properties["endpoint"] = "/hello"
				telemetryClient.Track(metric)
			}
		})
		
		v1.GET("/config", func(c *gin.Context) {
			if telemetryClient != nil {
				event := appinsights.NewEventTelemetry("config_endpoint_accessed")
				event.Properties["debug_mode"] = "true"
				telemetryClient.Track(event)
			}
			
			response := gin.H{
				"app": gin.H{
					"name":        config.Name,
					"version":     config.Version,
				},
				"telemetry": gin.H{
					"enabled":             config.ConnectionString != "",
					"instrumentation_key": parseInstrumentationKey(config.ConnectionString),
				},
			}
			
			c.JSON(200, response)
		})
		
	}

	address := fmt.Sprintf("%s:%d", "0.0.0.0", 8080)
	log.Printf("Servidor iniciado en %s con Application Insights habilitado", address)
	r.Run(address)
}
