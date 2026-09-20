package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/KeaganFerrao/adaptive-control-plane/internal/action_runner"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/actions_registry"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/config"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/control_plane"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/database"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/metric_collector"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/opamp_server"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/opamp_server/data"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/opamp_server/ui"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/policy_evaluator"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/policy_manager"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/repository"
	"github.com/prometheus/client_golang/api"
)

var logger = log.New(log.Default().Writer(), "[MAIN] ", log.Default().Flags()|log.Lmsgprefix|log.Lmicroseconds)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration: ", err)
	}

	pool, err := database.ConnectPostgres(ctx, cfg.DatabaseURl)
	if err != nil {
		logger.Fatal("Failed to connect to postgres: ", err)
	}
	defer pool.Close()

	policyRepository := repository.NewPostgresPolicyRepository(pool)
	componentRepository := repository.NewPostgresComponentRepository(pool)
	actionRegistry, err := actions_registry.GetRegistry()
	if err != nil {
		logger.Fatal("Failed to initialize action registry: ", err)
	}

	policyManager := policymanager.NewYAMLPolicyManager(cfg.PolicyFilePath, policyRepository, componentRepository, actionRegistry)
	policy, err := policyManager.Load(ctx)
	if err != nil {
		logger.Fatal("Failed to load policy: ", err)
	}

	for _, p := range policy {
		err = policyManager.Save(ctx, p)
		if err != nil {
			logger.Fatal("Failed to save policy: ", err)
		}
	}

	client, err := api.NewClient(api.Config{
		Address: cfg.PrometheusEndpoint,
	})
	if err != nil {
		logger.Fatal("Failed to create prometheus client: ", err)
	}

	actionRunner := action_runner.NewActionRunner()

	metricCollector := metric_collector.NewPrometheusMetricCollector(&client)
	policyEvaluator := policy_evaluator.NewPolicyEvaluator()
	controlPlane := control_plane.NewControlPlane(policy, metricCollector, actionRunner, policyEvaluator, 10*time.Second)

	if cfg.StartControlPlane {
		stopper := controlPlane.Start(ctx)
		defer stopper()
	} else {
		logger.Println("Control plane loop is disabled from configuration")
	}

	curDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	logger.Println("OpAMP Server starting...")

	ui.Start(curDir)
	defer ui.Shutdown()

	opampSrv := opamp_server.NewServer(data.AllAgents, false)
	opampSrv.Start(false)
	defer opampSrv.Stop()

	logger.Println("OpAMP Server running...")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	<-interrupt
}
