package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Financial-Times/smartlogic-notifier/kafka"
	"github.com/Financial-Times/smartlogic-notifier/notifier"
	"github.com/Financial-Times/smartlogic-notifier/smartlogic"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/jawher/mow.cli"
	_ "github.com/joho/godotenv/autoload"
)

const appDescription = "Entrypoint for concept publish notifications from the Smartlogic Semaphore system"

func main() {
	app := cli.App("smartlogic-notifier", appDescription)

	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "smartlogic-notifier",
		Desc:   "System Code of the application",
		EnvVar: "APP_SYSTEM_CODE",
	})

	appName := app.String(cli.StringOpt{
		Name:   "app-name",
		Value:  "Smartlogic Notifier",
		Desc:   "Application name",
		EnvVar: "APP_NAME",
	})

	kafkaAddresses := app.String(cli.StringOpt{
		Name:   "kafkaAddresses",
		Desc:   "Comma separated list of Kafka broker addresses",
		EnvVar: "KAFKA_ADDRESSES",
	})

	kafkaTopic := app.String(cli.StringOpt{
		Name:   "kafkaTopic",
		Desc:   "Kafka topic to send messages to",
		EnvVar: "KAFKA_TOPIC",
	})

	smartlogicBaseURL := app.String(cli.StringOpt{
		Name:   "smartlogicBaseURL",
		Desc:   "Base URL for the Smartlogic instance",
		EnvVar: "SMARTLOGIC_BASE_URL",
	})

	smartlogicModel := app.String(cli.StringOpt{
		Name:   "smartlogicModel",
		Desc:   "Smartlogic model to read from",
		EnvVar: "SMARTLOGIC_MODEL",
	})

	smartlogicAPIKey := app.String(cli.StringOpt{
		Name:   "smartlogicAPIKey",
		Desc:   "Smartlogic model to read from",
		EnvVar: "SMARTLOGIC_API_KEY",
	})

	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
		Desc:   "Port to listen on",
		EnvVar: "APP_PORT",
	})

	logLevel := app.String(cli.StringOpt{
		Name:   "logLevel",
		Value:  "info",
		Desc:   "Level of logging to be shown",
		EnvVar: "LOG_LEVEL",
	})

	lvl, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Warnf("Log level %s could not be parsed, defaulting to info")
		lvl = log.InfoLevel
	}
	log.SetLevel(lvl)
	log.Infof("[Startup] %s is starting", *appSystemCode)

	app.Action = func() {
		log.Infof("System code: %s, App Name: %s, Port: %s", *appSystemCode, *appName, *port)

		router := mux.NewRouter()
		go func() {
			if err := http.ListenAndServe(":"+*port, router); err != nil {
				log.Fatalf("Unable to start: %v", err)
			}
		}()

		kf, err := kafka.NewKafkaClient(*kafkaAddresses, *kafkaTopic)
		if err != nil {
			log.WithField("kafkaAddresses", *kafkaAddresses).WithField("kafkaTopic", *kafkaTopic).Fatalf("Error creating the Kafka producer.")
		}
		sl, err := smartlogic.NewSmartlogicClient(*smartlogicBaseURL, *smartlogicModel, *smartlogicAPIKey)
		if err != nil {
			log.Error("Error generating access token when connecting to Smartlogic.")
			log.WithFields(log.Fields{})
		}

		//concept, _ := sl.GetConcept("2d3e16e0-61cb-4322-8aff-3b01c59f4daa")
		//log.Info(string(concept))

		handler := notifier.NewNotifierHandler(kf, sl)
		handler.RegisterAdminEndpoints(router, *appSystemCode, *appName, appDescription)
		handler.RegisterEndpoints(router)

		waitForSignal()
	}
	err = app.Run(os.Args)
	if err != nil {
		log.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

func waitForSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
