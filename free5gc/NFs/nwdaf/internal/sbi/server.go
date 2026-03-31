package sbi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/nwdaf/internal/sbi/consumer"
	"github.com/free5gc/nwdaf/internal/sbi/processor"
	"github.com/free5gc/nwdaf/pkg/app"
	"github.com/free5gc/nwdaf/pkg/factory"
	"github.com/free5gc/util/httpwrapper"
	logger_util "github.com/free5gc/util/logger"
)

type ServerNwdaf interface {
	app.App

	Consumer() *consumer.Consumer
	Processor() *processor.Processor
}

type Server struct {
	ServerNwdaf

	httpServer *http.Server
	router     *gin.Engine
}

func NewServer(nwdaf ServerNwdaf, tlsKeyLogPath string) (*Server, error) {
	s := &Server{
		ServerNwdaf: nwdaf,
	}

	s.router = newRouter(s)

	cfg := s.Config()
	bindAddr := cfg.GetSbiBindingAddr()
	logger.SBILog.Infof("Binding addr: [%s]", bindAddr)

	var err error
	if s.httpServer, err = httpwrapper.NewHttp2Server(bindAddr, tlsKeyLogPath, s.router); err != nil {
		logger.SBILog.Errorf("Initialize HTTP server failed: %v", err)
		return nil, err
	}
	s.httpServer.ErrorLog = log.New(logger.SBILog.WriterLevel(logrus.ErrorLevel), "HTTP2: ", 0)

	return s, nil
}

func newRouter(s *Server) *gin.Engine {
	router := logger_util.NewGinWithLogrus(logger.GinLog)

	analyticsGroup := router.Group(factory.NwdafAnalyticsInfoResUriPrefix)
	applyRoutes(analyticsGroup, s.getAnalyticsInfoRoutes())

	eventsSubGroup := router.Group(factory.NwdafEventsSubscriptionResUriPrefix)
	applyRoutes(eventsSubGroup, s.getEventsSubscriptionRoutes())

	managementGroup := router.Group(factory.NwdafManagementResUriPrefix)
	applyRoutes(managementGroup, s.getManagementRoutes())

	callbackGroup := router.Group(factory.NwdafCallbackResUriPrefix)
	applyRoutes(callbackGroup, s.getCallbackRoutes())

	return router
}

func (s *Server) Run(traceCtx context.Context, wg *sync.WaitGroup) error {
	profile, err := s.Consumer().BuildNFInstance(s.Context())
	if err != nil {
		logger.SBILog.Warnf("Build NWDAF Profile Error: %+v", err)
	} else {
		_, retrievedNfId, regErr := s.Consumer().SendRegisterNFInstance(
			s.Context().NrfUri, s.Context().NfId, profile)
		if regErr != nil {
			logger.SBILog.Warnf("Send Register NF Instance failed: %+v", regErr)
		} else if retrievedNfId != "" {
			s.Context().NfId = retrievedNfId
		}
	}

	// Subscribe to AMF and SMF events after NRF registration
	go s.subscribeToNfEvents()

	wg.Add(1)
	go s.startServer(wg)

	return nil
}

func (s *Server) subscribeToNfEvents() {
	// Small delay to ensure the HTTP server is ready to receive callbacks
	time.Sleep(2 * time.Second)

	cfg := s.Config()
	eventSubCfg := cfg.GetEventSubscription()
	if eventSubCfg == nil {
		logger.SBILog.Infof("No event subscription configuration found, skipping")
		return
	}

	// Subscribe to AMF events
	if amfCfg := eventSubCfg.AmfEventSubscription; amfCfg != nil {
		logger.SBILog.Infof("Subscribing to AMF events at %s", amfCfg.NfUri)
		subId, err := s.Consumer().SubscribeAmfEvents(amfCfg.NfUri, amfCfg.Events)
		if err != nil {
			logger.SBILog.Warnf("Failed to subscribe to AMF events: %v", err)
		} else {
			logger.SBILog.Infof("AMF event subscription successful, id=%s", subId)
		}
	}

	// Subscribe to SMF events
	if smfCfg := eventSubCfg.SmfEventSubscription; smfCfg != nil {
		logger.SBILog.Infof("Subscribing to SMF events at %s", smfCfg.NfUri)
		subId, err := s.Consumer().SubscribeSmfEvents(smfCfg.NfUri, smfCfg.Events)
		if err != nil {
			logger.SBILog.Warnf("Failed to subscribe to SMF events: %v", err)
		} else {
			logger.SBILog.Infof("SMF event subscription successful, id=%s", subId)
		}
	}
}

func (s *Server) Stop() {
	const defaultShutdownTimeout time.Duration = 2 * time.Second

	if s.httpServer != nil {
		logger.SBILog.Infof("Stop SBI server (listen on %s)", s.httpServer.Addr)
		toCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		if err := s.httpServer.Shutdown(toCtx); err != nil {
			logger.SBILog.Errorf("Could not close SBI server: %#v", err)
		}
	}
}

func (s *Server) startServer(wg *sync.WaitGroup) {
	defer func() {
		if p := recover(); p != nil {
			logger.SBILog.Fatalf("panic: %v\n%s", p, string(debug.Stack()))
			s.Terminate()
		}
		wg.Done()
	}()

	logger.SBILog.Infof("Start SBI server (listen on %s)", s.httpServer.Addr)

	var err error
	cfg := s.Config()
	scheme := cfg.GetSbiScheme()
	if scheme == "http" {
		err = s.httpServer.ListenAndServe()
	} else if scheme == "https" {
		err = s.httpServer.ListenAndServeTLS(
			cfg.GetCertPemPath(),
			cfg.GetCertKeyPath())
	} else {
		err = fmt.Errorf("no support this scheme[%s]", scheme)
	}

	if err != nil && err != http.ErrServerClosed {
		logger.SBILog.Errorf("SBI server error: %v", err)
	}
	logger.SBILog.Warnf("SBI server (listen on %s) stopped", s.httpServer.Addr)
}
