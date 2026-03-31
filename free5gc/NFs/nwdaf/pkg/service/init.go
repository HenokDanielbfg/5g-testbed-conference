package service

import (
	"context"
	"io"
	"os"
	"runtime/debug"
	"sync"

	"github.com/sirupsen/logrus"

	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/nwdaf/internal/sbi"
	"github.com/free5gc/nwdaf/internal/sbi/consumer"
	"github.com/free5gc/nwdaf/internal/sbi/processor"
	"github.com/free5gc/nwdaf/pkg/app"
	"github.com/free5gc/nwdaf/pkg/factory"
)

type NwdafAppInterface interface {
	app.App
	Consumer() *consumer.Consumer
	Processor() *processor.Processor
}

var NWDAF NwdafAppInterface

type NwdafApp struct {
	NwdafAppInterface

	cfg      *factory.Config
	nwdafCtx *nwdaf_context.NWDAFContext
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	processor *processor.Processor
	consumer  *consumer.Consumer
	sbiServer *sbi.Server
}

func NewApp(ctx context.Context, cfg *factory.Config, tlsKeyLogPath string) (*NwdafApp, error) {
	nwdaf := &NwdafApp{cfg: cfg}
	nwdaf.SetLogEnable(cfg.GetLogEnable())
	nwdaf.SetLogLevel(cfg.GetLogLevel())
	nwdaf.SetReportCaller(cfg.GetLogReportCaller())

	c, err := consumer.NewConsumer(nwdaf)
	if err != nil {
		return nwdaf, err
	}
	nwdaf.consumer = c

	p, err := processor.NewProcessor(nwdaf)
	if err != nil {
		return nwdaf, err
	}
	nwdaf.processor = p

	nwdaf.ctx, nwdaf.cancel = context.WithCancel(ctx)
	nwdaf.nwdafCtx = nwdaf_context.GetSelf()

	if nwdaf.sbiServer, err = sbi.NewServer(nwdaf, tlsKeyLogPath); err != nil {
		return nil, err
	}

	NWDAF = nwdaf
	return nwdaf, nil
}

func (a *NwdafApp) SetLogEnable(enable bool) {
	logger.MainLog.Infof("Log enable is set to [%v]", enable)
	if enable && logger.Log.Out == os.Stderr {
		return
	} else if !enable && logger.Log.Out == io.Discard {
		return
	}

	a.cfg.SetLogEnable(enable)
	if enable {
		logger.Log.SetOutput(os.Stderr)
	} else {
		logger.Log.SetOutput(io.Discard)
	}
}

func (a *NwdafApp) SetLogLevel(level string) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		logger.MainLog.Warnf("Log level [%s] is invalid", level)
		return
	}

	logger.MainLog.Infof("Log level is set to [%s]", level)
	if lvl == logger.Log.GetLevel() {
		return
	}

	a.cfg.SetLogLevel(level)
	logger.Log.SetLevel(lvl)
}

func (a *NwdafApp) SetReportCaller(reportCaller bool) {
	logger.MainLog.Infof("Report Caller is set to [%v]", reportCaller)
	if reportCaller == logger.Log.ReportCaller {
		return
	}

	a.cfg.SetLogReportCaller(reportCaller)
	logger.Log.SetReportCaller(reportCaller)
}

func (a *NwdafApp) Start() {
	nwdaf_context.InitNwdafContext(a.nwdafCtx)
	logger.InitLog.Infoln("Server started")

	if err := a.sbiServer.Run(context.Background(), &a.wg); err != nil {
		logger.MainLog.Fatalf("Run SBI server failed: %+v", err)
	}

	a.wg.Add(1)
	go a.listenShutdownEvent()

	a.WaitRoutineStopped()
}

func (a *NwdafApp) Terminate() {
	a.cancel()
}

func (a *NwdafApp) Config() *factory.Config {
	return a.cfg
}

func (a *NwdafApp) Context() *nwdaf_context.NWDAFContext {
	return a.nwdafCtx
}

func (a *NwdafApp) CancelContext() context.Context {
	return a.ctx
}

func (a *NwdafApp) Consumer() *consumer.Consumer {
	return a.consumer
}

func (a *NwdafApp) Processor() *processor.Processor {
	return a.processor
}

func (a *NwdafApp) listenShutdownEvent() {
	defer func() {
		if p := recover(); p != nil {
			logger.MainLog.Fatalf("panic: %v\n%s", p, string(debug.Stack()))
		}
		a.wg.Done()
	}()

	<-a.ctx.Done()
	a.terminateProcedure()
}

func (a *NwdafApp) CallServerStop() {
	if a.sbiServer != nil {
		a.sbiServer.Stop()
	}
}

func (a *NwdafApp) WaitRoutineStopped() {
	a.wg.Wait()
	logger.MainLog.Infof("NWDAF App is terminated")
}

func (a *NwdafApp) terminateProcedure() {
	logger.MainLog.Infof("Terminating NWDAF...")

	// Unsubscribe from NF events before shutting down
	a.unsubscribeFromNfEvents()

	a.CallServerStop()

	problemDetails, err := a.Consumer().SendDeregisterNFInstance()
	if problemDetails != nil {
		logger.MainLog.Errorf("Deregister NF instance Failed Problem[%+v]", problemDetails)
	} else if err != nil {
		logger.MainLog.Errorf("Deregister NF instance Error[%+v]", err)
	} else {
		logger.MainLog.Infof("[NWDAF] Deregister from NRF successfully")
	}
}

func (a *NwdafApp) unsubscribeFromNfEvents() {
	eventSubCfg := a.cfg.GetEventSubscription()
	if eventSubCfg == nil {
		return
	}

	nwdafCtx := a.nwdafCtx

	// Unsubscribe from AMF events
	if amfCfg := eventSubCfg.AmfEventSubscription; amfCfg != nil && nwdafCtx.AmfSubscriptionId != "" {
		if err := a.Consumer().UnsubscribeAmfEvents(amfCfg.NfUri, nwdafCtx.AmfSubscriptionId); err != nil {
			logger.MainLog.Warnf("Failed to unsubscribe from AMF events: %v", err)
		}
	}

	// Unsubscribe from SMF events
	if smfCfg := eventSubCfg.SmfEventSubscription; smfCfg != nil && nwdafCtx.SmfSubscriptionId != "" {
		if err := a.Consumer().UnsubscribeSmfEvents(smfCfg.NfUri, nwdafCtx.SmfSubscriptionId); err != nil {
			logger.MainLog.Warnf("Failed to unsubscribe from SMF events: %v", err)
		}
	}
}
