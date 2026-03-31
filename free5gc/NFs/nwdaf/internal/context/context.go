package context

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/nwdaf/pkg/factory"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/openapi/oauth"
)

const (
	NwdafEvent_NF_LOAD             = "NF_LOAD"
	NwdafEvent_NETWORK_PERFORMANCE = "NETWORK_PERFORMANCE"
	NwdafEvent_UE_MOBILITY         = "UE_MOBILITY"
	NwdafEvent_QOS_SUSTAINABILITY  = "QOS_SUSTAINABILITY"
	NwdafEvent_SLICE_LOAD_LEVEL    = "SLICE_LOAD_LEVEL"
	NwdafEvent_UE_COMMUNICATION    = "UE_COMMUNICATION"
	NwdafEvent_ABNORMAL_BEHAVIOUR  = "ABNORMAL_BEHAVIOUR"
	NwdafEvent_RECOMMENDATION      = "NWDAF_RECOMMENDATION"
)

type NWDAFContext struct {
	NfId           string
	NrfUri         string
	NrfCertPem     string
	RegisterIPv4   string
	BindingIPv4    string
	SBIPort        int
	UriScheme      models.UriScheme
	NfService      map[models.ServiceName]models.NfService
	OAuth2Required bool
	Subscriptions  sync.Map // key: subscriptionId (string), value: *NwdafSubscription

	// Outbound event subscriptions (NWDAF subscribing TO other NFs)
	AmfSubscriptionId string // subscription ID returned by AMF
	SmfSubscriptionId string // subscription ID returned by SMF

	// Received events store (latest per type for backward compat)
	AmfEvents sync.Map // key: eventType (string), value: *ReceivedAmfEvent
	SmfEvents sync.Map // key: eventType (string), value: *ReceivedSmfEvent

	// Event history store (ring buffer + per-UE tracking)
	EventStore *EventStore

	// Process metrics collector (samples /proc for CPU and memory)
	ProcMetrics *ProcMetricsCollector

	// Baseline store (epoch-based registration rate history for anomaly detection)
	Baseline *BaselineStore
}

type NwdafSubscription struct {
	SubscriptionId string
	NfConsumerUri  string
	Events         []string
	Expiry         *time.Time
}

type ReceivedAmfEvent struct {
	EventType           string
	Timestamp           time.Time
	NotifyCorrelationId string
	Reports             []interface{} // raw AmfEventReport data
	Supi                string
	Location            *models.UserLocation
	Timezone            string
	Reachability        string
	CmInfoList          []models.CmInfo
	RmInfoList          []models.RmInfo
	StateActive         *bool // from report.State.Active (AMF uses this for REGISTRATION_STATE_REPORT)
}

type ReceivedSmfEvent struct {
	EventType  string
	Timestamp  time.Time
	NotifId    string
	Supi       string
	PduSeId    int32
	SourceDnai string
	TargetDnai string
	UeIpv4     string
}

var nwdafContext NWDAFContext

func init() {
	GetSelf().NfService = make(map[models.ServiceName]models.NfService)
	GetSelf().UriScheme = models.UriScheme_HTTPS
}

func GetSelf() *NWDAFContext {
	return &nwdafContext
}

func InitNwdafContext(nwdafCtx *NWDAFContext) {
	config := factory.NwdafConfig
	logger.InitLog.Infof("nwdafconfig Info: Version[%s] Description[%s]",
		config.Info.Version, config.Info.Description)

	nwdafCtx.NfId = uuid.New().String()
	nwdafCtx.NrfUri = config.GetNrfUri()

	if config.Configuration != nil {
		nwdafCtx.NrfCertPem = config.Configuration.NrfCertPem
	}

	nwdafCtx.RegisterIPv4 = config.GetSbiRegisterIP()
	nwdafCtx.BindingIPv4 = config.GetSbiBindingIP()
	nwdafCtx.SBIPort = config.GetSbiPort()
	nwdafCtx.UriScheme = models.UriScheme(config.GetSbiScheme())
	nwdafCtx.OAuth2Required = false
	nwdafCtx.EventStore = NewEventStore()
	nwdafCtx.ProcMetrics = NewProcMetricsCollector(5 * time.Second)
	// Epoch every 30s, retain 30 epochs (~15 min of baseline history).
	nwdafCtx.Baseline = NewBaselineStore(nwdafCtx.EventStore, 30*time.Second, 30)

	serviceNameList := config.GetServiceNameList()
	nwdafCtx.initNFService(serviceNameList, config.GetVersion())
}

func (c *NWDAFContext) initNFService(serviceNameList []string, version string) {
	versionUri := "v1"
	if len(version) > 0 {
		versionUri = "v" + string(version[0])
	}

	for index, nameString := range serviceNameList {
		name := models.ServiceName(nameString)
		c.NfService[name] = models.NfService{
			ServiceInstanceId: strconv.Itoa(index),
			ServiceName:       name,
			Versions: &[]models.NfServiceVersion{
				{
					ApiFullVersion:  version,
					ApiVersionInUri: versionUri,
				},
			},
			Scheme:          c.UriScheme,
			NfServiceStatus: models.NfServiceStatus_REGISTERED,
			ApiPrefix:       c.GetIPv4Uri(),
			IpEndPoints: &[]models.IpEndPoint{
				{
					Ipv4Address: c.RegisterIPv4,
					Transport:   models.TransportProtocol_TCP,
					Port:        int32(c.SBIPort),
				},
			},
		}
	}
}

func (c *NWDAFContext) GetIPv4Uri() string {
	return fmt.Sprintf("%s://%s:%d", c.UriScheme, c.RegisterIPv4, c.SBIPort)
}

func (c *NWDAFContext) GetTokenCtx(serviceName models.ServiceName, targetNf models.NfType) (
	context.Context, *models.ProblemDetails, error,
) {
	if !c.OAuth2Required {
		return context.TODO(), nil, nil
	}
	return oauth.GetTokenCtx(models.NfType_NWDAF, targetNf,
		c.NfId, c.NrfUri, string(serviceName))
}
