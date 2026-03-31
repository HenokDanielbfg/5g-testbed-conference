package app

import (
	"context"

	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/pkg/factory"
)

type App interface {
	SetLogEnable(enable bool)
	SetLogLevel(level string)
	SetReportCaller(reportCaller bool)

	Start()
	Terminate()

	Config() *factory.Config
	Context() *nwdaf_context.NWDAFContext
	CancelContext() context.Context
}
