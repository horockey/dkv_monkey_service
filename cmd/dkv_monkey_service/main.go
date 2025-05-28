package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/horockey/dkv"
	"github.com/horockey/dkv_monkey_service/internal/model"
	"github.com/horockey/go-toolbox/prometheus_server"
	serdisc "github.com/horockey/service_discovery/api"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/rs/zerolog"
)

const TotalStorageCap = 1_000_000

func main() {
	serv := http.Server{
		Addr: "0.0.0.0:80", // TODO: from cfg
	}

	logger := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	disc, err := serdisc.NewClient(
		"dkv_monkey_service",    // TODO: to const
		"http://discovery:6500", // TODO: from cfg
		"ak",                    // TODO: from cfg
		&serv,
		logger.
			With().
			Str("scope", "serdisc_client").
			Logger(),
	)
	if err != nil {
		logger.
			Fatal().
			Err(fmt.Errorf("creating serdisc client: %w", err)).
			Send()
	}

	hostname, _ := os.Hostname()
	cl, err := dkv.NewClient(
		"dkv_ak", // TODO: from cfg
		hostname, // TODO: from cfg
		&model.DiscoveryImpl{Cl: disc, Logger: logger.With().Str("scope", "dkv_cl_impl").Logger()},
		dkv.WithServicePort[model.Value](7000),
		dkv.WithLogger[model.Value](
			logger.
				With().
				Str("scope", "dkv_client").
				Logger(),
		),
	)
	if err != nil {
		logger.
			Fatal().
			Err(fmt.Errorf("creating dkv client: %w", err)).
			Send()
	}

	ps, err := prometheus_server.New("", prometheus_server.WithServer(&serv))
	if err != nil {
		logger.
			Fatal().
			Err(fmt.Errorf("creating prometheus server: %w", err)).
			Send()
	}

	if err := ps.Register(
		append(
			cl.Metrics(),
			collectors.NewGoCollector(),
		)...,
	); err != nil {
		logger.
			Fatal().
			Err(fmt.Errorf("registering metrics: %w", err)).
			Send()
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		syscall.SIGABRT,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGKILL,
	)
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := serv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.
				Error().
				Err(fmt.Errorf("running http_server")).
				Send()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := cl.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.
				Error().
				Err(fmt.Errorf("running dkv client: %w", err)).
				Send()
			cancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := ps.Start(ctx); err != nil {
			logger.
				Error().
				Err(fmt.Errorf("prometheus server: %w", err)).
				Send()
			cancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		time.Sleep(time.Second * 5)

		insertCnt := 100
		if v := os.Getenv("KEYS_COUNT"); v != "" {
			cnt := 0
			cnt, err = strconv.Atoi(v)
			if err == nil {
				insertCnt = cnt
			}
		}

		for range insertCnt {
			key := "monkey_" + uuid.NewString()
			value := model.Value{
				Foo: uuid.NewString(),
				Bar: uuid.NewString(),
			}
			if err := cl.AddOrUpdate(ctx, key, value); err != nil {
				logger.Error().Err(fmt.Errorf("writing to client: %w", err)).Send()
			}
		}
	}()

	logger.Info().Msg("Service started")

	<-ctx.Done()
	_ = serv.Close()
	wg.Wait()

	logger.Info().Msg("Service stopped")
}
