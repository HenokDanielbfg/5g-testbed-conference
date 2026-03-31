package consumer

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/openapi"
	"github.com/free5gc/openapi/Nnrf_NFDiscovery"
	"github.com/free5gc/openapi/Nnrf_NFManagement"
	"github.com/free5gc/openapi/models"
)

type nnrfService struct {
	consumer        *Consumer
	nfMngmntMu      sync.RWMutex
	nfDiscMu        sync.RWMutex
	nfMngmntClients map[string]*Nnrf_NFManagement.APIClient
	nfDiscClients   map[string]*Nnrf_NFDiscovery.APIClient
}

func (s *nnrfService) getNFManagementClient(uri string) *Nnrf_NFManagement.APIClient {
	if uri == "" {
		return nil
	}
	s.nfMngmntMu.RLock()
	client, ok := s.nfMngmntClients[uri]
	if ok {
		s.nfMngmntMu.RUnlock()
		return client
	}
	configuration := Nnrf_NFManagement.NewConfiguration()
	configuration.SetBasePath(uri)
	client = Nnrf_NFManagement.NewAPIClient(configuration)
	s.nfMngmntMu.RUnlock()
	s.nfMngmntMu.Lock()
	defer s.nfMngmntMu.Unlock()
	s.nfMngmntClients[uri] = client
	return client
}

func (s *nnrfService) getNFDiscClient(uri string) *Nnrf_NFDiscovery.APIClient {
	if uri == "" {
		return nil
	}
	s.nfDiscMu.RLock()
	client, ok := s.nfDiscClients[uri]
	if ok {
		s.nfDiscMu.RUnlock()
		return client
	}
	configuration := Nnrf_NFDiscovery.NewConfiguration()
	configuration.SetBasePath(uri)
	client = Nnrf_NFDiscovery.NewAPIClient(configuration)
	s.nfDiscMu.RUnlock()
	s.nfDiscMu.Lock()
	defer s.nfDiscMu.Unlock()
	s.nfDiscClients[uri] = client
	return client
}

func (s *nnrfService) BuildNFInstance(nwdafContext *nwdaf_context.NWDAFContext) (profile models.NfProfile, err error) {
	profile.NfInstanceId = nwdafContext.NfId
	profile.NfType = models.NfType_NWDAF
	profile.NfStatus = models.NfStatus_REGISTERED
	if nwdafContext.RegisterIPv4 == "" {
		err = fmt.Errorf("NWDAF Address is empty")
		return profile, err
	}
	profile.Ipv4Addresses = append(profile.Ipv4Addresses, nwdafContext.RegisterIPv4)
	services := []models.NfService{}
	for _, nfService := range nwdafContext.NfService {
		services = append(services, nfService)
	}
	if len(services) > 0 {
		profile.NfServices = &services
	}
	return profile, err
}

func (s *nnrfService) SendRegisterNFInstance(nrfUri, nfInstanceId string, profile models.NfProfile) (
	resourceNrfUri string, retrieveNfInstanceId string, err error,
) {
	client := s.getNFManagementClient(nrfUri)
	if client == nil {
		return "", "", openapi.ReportError("nrf not found")
	}

	var res *http.Response
	var nf models.NfProfile
	for {
		nf, res, err = client.NFInstanceIDDocumentApi.RegisterNFInstance(context.TODO(), nfInstanceId, profile)
		if err != nil || res == nil {
			logger.ConsumerLog.Errorf("NWDAF register to NRF Error[%v]", err)
			time.Sleep(2 * time.Second)
			continue
		}
		defer func() {
			if res != nil {
				if bodyCloseErr := res.Body.Close(); bodyCloseErr != nil {
					err = fmt.Errorf("RegisterNFInstance response body cannot close: %+w", bodyCloseErr)
				}
			}
		}()
		status := res.StatusCode
		if status == http.StatusOK {
			// NFUpdate
			break
		} else if status == http.StatusCreated {
			// NFRegister
			resourceUri := res.Header.Get("Location")
			index := strings.Index(resourceUri, "/nnrf-nfm/")
			if index >= 0 {
				resourceNrfUri = resourceUri[:index]
			}
			retrieveNfInstanceId = resourceUri[strings.LastIndex(resourceUri, "/")+1:]

			oauth2 := false
			if nf.CustomInfo != nil {
				v, ok := nf.CustomInfo["oauth2"].(bool)
				if ok {
					oauth2 = v
					logger.ConsumerLog.Infoln("OAuth2 setting receive from NRF:", oauth2)
				}
			}
			nwdaf_context.GetSelf().OAuth2Required = oauth2
			if oauth2 && nwdaf_context.GetSelf().NrfCertPem == "" {
				logger.ConsumerLog.Error("OAuth2 enable but no nrfCertPem provided in config.")
			}
			break
		} else {
			logger.ConsumerLog.Errorf("NRF return wrong status code %d", status)
		}
	}
	return resourceNrfUri, retrieveNfInstanceId, err
}

func (s *nnrfService) SendDeregisterNFInstance() (problemDetails *models.ProblemDetails, err error) {
	logger.ConsumerLog.Infof("[NWDAF] Send Deregister NFInstance")

	nwdafContext := s.consumer.Context()
	client := s.getNFManagementClient(nwdafContext.NrfUri)
	if client == nil {
		return nil, openapi.ReportError("nrf not found")
	}

	ctx, pd, err := nwdaf_context.GetSelf().GetTokenCtx(models.ServiceName_NNRF_NFM, models.NfType_NRF)
	if err != nil {
		return pd, err
	}

	var res *http.Response
	res, err = client.NFInstanceIDDocumentApi.DeregisterNFInstance(ctx, nwdafContext.NfId)
	if err == nil {
		return problemDetails, err
	} else if res != nil {
		defer func() {
			if bodyCloseErr := res.Body.Close(); bodyCloseErr != nil {
				err = fmt.Errorf("DeregisterNFInstance response body cannot close: %+w", bodyCloseErr)
			}
		}()
		if res.Status != err.Error() {
			return problemDetails, err
		}
		problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("server no response")
	}
	return problemDetails, err
}

func (s *nnrfService) SendNFDiscoveryToNrf(targetNfType models.NfType) (*models.SearchResult, error) {
	nwdafContext := s.consumer.Context()

	client := s.getNFDiscClient(nwdafContext.NrfUri)
	if client == nil {
		return nil, openapi.ReportError("nrf not found")
	}

	ctx, _, err := nwdaf_context.GetSelf().GetTokenCtx(models.ServiceName_NNRF_DISC, models.NfType_NRF)
	if err != nil {
		return nil, err
	}

	result, res, err := client.NFInstancesStoreApi.SearchNFInstances(
		ctx, targetNfType, models.NfType_NWDAF, nil)
	if res != nil && res.StatusCode == http.StatusTemporaryRedirect {
		return nil, fmt.Errorf("temporary Redirect For Non NRF Consumer")
	}
	if res == nil || res.Body == nil {
		return &result, err
	}
	defer func() {
		if res != nil {
			if bodyCloseErr := res.Body.Close(); bodyCloseErr != nil {
				err = fmt.Errorf("SearchNFInstances response body cannot close: %+w", bodyCloseErr)
			}
		}
	}()
	return &result, err
}
