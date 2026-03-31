package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"

	"github.com/urfave/cli"

	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/nwdaf/pkg/factory"
	"github.com/free5gc/nwdaf/pkg/service"
	logger_util "github.com/free5gc/util/logger"
	"github.com/free5gc/util/version"
)

var NWDAF *service.NwdafApp

func main() {
	defer func() {
		if p := recover(); p != nil {
			logger.MainLog.Fatalf("panic: %v\n%s", p, string(debug.Stack()))
		}
	}()

	app := cli.NewApp()
	app.Name = "nwdaf"
	app.Usage = "5G Network Data Analytics Function (NWDAF)"
	app.Action = action
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Usage: "Load configuration from `FILE`",
		},
		cli.StringSliceFlag{
			Name:  "log, l",
			Usage: "Output NF log to `FILE`",
		},
	}
	if err := app.Run(os.Args); err != nil {
		logger.MainLog.Errorf("NWDAF Run error: %v\n", err)
		return
	}
}

func action(cliCtx *cli.Context) error {
	tlsKeyLogPath, err := initLogFile(cliCtx.StringSlice("log"))
	if err != nil {
		return err
	}

	logger.MainLog.Infoln("NWDAF version: ", version.GetVersion())

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
	}()

	cfg, err := factory.ReadConfig(cliCtx.String("config"))
	if err != nil {
		return err
	}
	factory.NwdafConfig = cfg

	nwdaf, err := service.NewApp(ctx, cfg, tlsKeyLogPath)
	if err != nil {
		return err
	}
	NWDAF = nwdaf

	nwdaf.Start()

	return nil
}

func initLogFile(logNfPath []string) (string, error) {
	logTlsKeyPath := ""

	for _, path := range logNfPath {
		if err := logger_util.LogFileHook(logger.Log, path); err != nil {
			return "", err
		}

		if logTlsKeyPath != "" {
			continue
		}

		nfDir, _ := filepath.Split(path)
		tmpDir := filepath.Join(nfDir, "key")
		if err := os.MkdirAll(tmpDir, 0o775); err != nil {
			logger.InitLog.Errorf("Make directory %s failed: %+v", tmpDir, err)
			return "", err
		}
		_, name := filepath.Split(factory.NwdafDefaultTLSKeyLogPath)
		logTlsKeyPath = filepath.Join(tmpDir, name)
	}

	return logTlsKeyPath, nil
}
