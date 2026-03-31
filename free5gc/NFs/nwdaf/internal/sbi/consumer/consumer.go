package consumer

import (
	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/pkg/factory"
	"github.com/free5gc/openapi/Namf_EventExposure"
	"github.com/free5gc/openapi/Nnrf_NFDiscovery"
	"github.com/free5gc/openapi/Nnrf_NFManagement"
	"github.com/free5gc/openapi/Nsmf_EventExposure"
	"github.com/free5gc/openapi/models"
)

type ConsumerNwdaf interface {
	Config() *factory.Config
	Context() *nwdaf_context.NWDAFContext
}

type Consumer struct {
	ConsumerNwdaf
	*nnrfService
	*namfService
	*nsmfService
}

func NewConsumer(nwdaf ConsumerNwdaf) (*Consumer, error) {
	c := &Consumer{
		ConsumerNwdaf: nwdaf,
	}
	c.nnrfService = &nnrfService{
		consumer:        c,
		nfMngmntClients: make(map[string]*Nnrf_NFManagement.APIClient),
		nfDiscClients:   make(map[string]*Nnrf_NFDiscovery.APIClient),
	}
	c.namfService = &namfService{
		consumer:   c,
		amfClients: make(map[string]*Namf_EventExposure.APIClient),
	}
	c.nsmfService = &nsmfService{
		consumer:   c,
		smfClients: make(map[string]*Nsmf_EventExposure.APIClient),
	}
	return c, nil
}

func (c *Consumer) BuildNFInstance(nwdafContext *nwdaf_context.NWDAFContext) (models.NfProfile, error) {
	return c.nnrfService.BuildNFInstance(nwdafContext)
}

func (c *Consumer) SendRegisterNFInstance(nrfUri, nfInstanceId string, profile models.NfProfile) (string, string, error) {
	return c.nnrfService.SendRegisterNFInstance(nrfUri, nfInstanceId, profile)
}

func (c *Consumer) SendDeregisterNFInstance() (*models.ProblemDetails, error) {
	return c.nnrfService.SendDeregisterNFInstance()
}

func (c *Consumer) SendNFDiscoveryToNrf(targetNfType models.NfType) (*models.SearchResult, error) {
	return c.nnrfService.SendNFDiscoveryToNrf(targetNfType)
}

func (c *Consumer) SubscribeAmfEvents(amfUri string, events []string) (string, error) {
	return c.namfService.SubscribeAmfEvents(amfUri, events)
}

func (c *Consumer) UnsubscribeAmfEvents(amfUri string, subscriptionId string) error {
	return c.namfService.UnsubscribeAmfEvents(amfUri, subscriptionId)
}

func (c *Consumer) SubscribeSmfEvents(smfUri string, events []string) (string, error) {
	return c.nsmfService.SubscribeSmfEvents(smfUri, events)
}

func (c *Consumer) UnsubscribeSmfEvents(smfUri string, subscriptionId string) error {
	return c.nsmfService.UnsubscribeSmfEvents(smfUri, subscriptionId)
}
