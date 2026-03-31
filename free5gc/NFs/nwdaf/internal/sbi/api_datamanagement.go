package sbi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/nwdaf/internal/sbi/processor"
	"github.com/free5gc/openapi/models"
)

// GetNfSubscriptions godoc
// GET /nnwdaf-management/v1/nf-subscriptions
// Returns the current outbound subscription status for AMF and SMF.
func (s *Server) GetNfSubscriptions(c *gin.Context) {
	status, pd := s.Processor().HandleGetNfSubscriptions()
	if pd != nil {
		c.JSON(int(pd.Status), pd)
		return
	}
	c.JSON(http.StatusOK, status)
}

// SubscribeAmfEvents godoc
// POST /nnwdaf-management/v1/nf-subscriptions/amf
// Subscribes NWDAF to AMF events. Body is optional; omitting it uses the
// default event list from nwdafcfg.yaml.
func (s *Server) SubscribeAmfEvents(c *gin.Context) {
	req := parseOptionalSubscribeRequest(c)
	code, body := s.handleSubscribe("amf", req)
	if body == nil {
		c.Status(code)
	} else {
		c.JSON(code, body)
	}
}

// UnsubscribeAmfEvents godoc
// DELETE /nnwdaf-management/v1/nf-subscriptions/amf
// Unsubscribes NWDAF from AMF events.
func (s *Server) UnsubscribeAmfEvents(c *gin.Context) {
	code, body := s.handleUnsubscribe("amf")
	if body == nil {
		c.Status(code)
	} else {
		c.JSON(code, body)
	}
}

// SubscribeSmfEvents godoc
// POST /nnwdaf-management/v1/nf-subscriptions/smf
// Subscribes NWDAF to SMF events. Body is optional.
func (s *Server) SubscribeSmfEvents(c *gin.Context) {
	req := parseOptionalSubscribeRequest(c)
	code, body := s.handleSubscribe("smf", req)
	if body == nil {
		c.Status(code)
	} else {
		c.JSON(code, body)
	}
}

// UnsubscribeSmfEvents godoc
// DELETE /nnwdaf-management/v1/nf-subscriptions/smf
// Unsubscribes NWDAF from SMF events.
func (s *Server) UnsubscribeSmfEvents(c *gin.Context) {
	code, body := s.handleUnsubscribe("smf")
	if body == nil {
		c.Status(code)
	} else {
		c.JSON(code, body)
	}
}

// handleSubscribe contains the shared subscribe logic for AMF and SMF.
func (s *Server) handleSubscribe(nfType string, req *processor.NfSubscribeRequest) (int, interface{}) {
	ctx := s.Context()
	cfg := s.Config()
	eventSubCfg := cfg.GetEventSubscription()

	if eventSubCfg == nil {
		return http.StatusUnprocessableEntity, &models.ProblemDetails{
			Status: http.StatusUnprocessableEntity,
			Title:  "No Event Subscription Configuration",
			Detail: "eventSubscription section missing from nwdafcfg.yaml",
		}
	}

	switch nfType {
	case "amf":
		if ctx.AmfSubscriptionId != "" {
			return http.StatusConflict, &models.ProblemDetails{
				Status: http.StatusConflict,
				Title:  "Already Subscribed",
				Detail: fmt.Sprintf("AMF subscription already active (id=%s); DELETE first", ctx.AmfSubscriptionId),
			}
		}
		amfCfg := eventSubCfg.AmfEventSubscription
		if amfCfg == nil {
			return http.StatusUnprocessableEntity, &models.ProblemDetails{
				Status: http.StatusUnprocessableEntity,
				Title:  "No AMF Configuration",
				Detail: "amfEventSubscription missing from nwdafcfg.yaml",
			}
		}
		events := amfCfg.Events
		if req != nil && len(req.Events) > 0 {
			events = req.Events
		}
		logger.SBILog.Infof("[Management] Subscribing to AMF events %v at %s", events, amfCfg.NfUri)
		subId, err := s.Consumer().SubscribeAmfEvents(amfCfg.NfUri, events)
		if err != nil {
			return http.StatusInternalServerError, &models.ProblemDetails{
				Status: http.StatusInternalServerError,
				Title:  "Subscription Failed",
				Detail: err.Error(),
			}
		}
		return http.StatusCreated, &processor.NfSubscriptionEntry{
			Subscribed:     true,
			SubscriptionId: subId,
			NfUri:          amfCfg.NfUri,
			Events:         events,
		}

	case "smf":
		if ctx.SmfSubscriptionId != "" {
			return http.StatusConflict, &models.ProblemDetails{
				Status: http.StatusConflict,
				Title:  "Already Subscribed",
				Detail: fmt.Sprintf("SMF subscription already active (id=%s); DELETE first", ctx.SmfSubscriptionId),
			}
		}
		smfCfg := eventSubCfg.SmfEventSubscription
		if smfCfg == nil {
			return http.StatusUnprocessableEntity, &models.ProblemDetails{
				Status: http.StatusUnprocessableEntity,
				Title:  "No SMF Configuration",
				Detail: "smfEventSubscription missing from nwdafcfg.yaml",
			}
		}
		events := smfCfg.Events
		if req != nil && len(req.Events) > 0 {
			events = req.Events
		}
		logger.SBILog.Infof("[Management] Subscribing to SMF events %v at %s", events, smfCfg.NfUri)
		subId, err := s.Consumer().SubscribeSmfEvents(smfCfg.NfUri, events)
		if err != nil {
			return http.StatusInternalServerError, &models.ProblemDetails{
				Status: http.StatusInternalServerError,
				Title:  "Subscription Failed",
				Detail: err.Error(),
			}
		}
		return http.StatusCreated, &processor.NfSubscriptionEntry{
			Subscribed:     true,
			SubscriptionId: subId,
			NfUri:          smfCfg.NfUri,
			Events:         events,
		}
	}

	return http.StatusBadRequest, &models.ProblemDetails{
		Status: http.StatusBadRequest,
		Detail: "unknown NF type: " + nfType,
	}
}

// handleUnsubscribe contains the shared unsubscribe logic for AMF and SMF.
func (s *Server) handleUnsubscribe(nfType string) (int, interface{}) {
	ctx := s.Context()
	cfg := s.Config()
	eventSubCfg := cfg.GetEventSubscription()

	if eventSubCfg == nil {
		return http.StatusUnprocessableEntity, &models.ProblemDetails{
			Status: http.StatusUnprocessableEntity,
			Title:  "No Event Subscription Configuration",
			Detail: "eventSubscription section missing from nwdafcfg.yaml",
		}
	}

	switch nfType {
	case "amf":
		if ctx.AmfSubscriptionId == "" {
			return http.StatusConflict, &models.ProblemDetails{
				Status: http.StatusConflict,
				Title:  "Not Subscribed",
				Detail: "No active AMF subscription to cancel",
			}
		}
		amfCfg := eventSubCfg.AmfEventSubscription
		if amfCfg == nil {
			return http.StatusUnprocessableEntity, &models.ProblemDetails{
				Status: http.StatusUnprocessableEntity,
				Detail: "amfEventSubscription missing from nwdafcfg.yaml",
			}
		}
		logger.SBILog.Infof("[Management] Unsubscribing from AMF events (id=%s)", ctx.AmfSubscriptionId)
		if err := s.Consumer().UnsubscribeAmfEvents(amfCfg.NfUri, ctx.AmfSubscriptionId); err != nil {
			return http.StatusInternalServerError, &models.ProblemDetails{
				Status: http.StatusInternalServerError,
				Title:  "Unsubscription Failed",
				Detail: err.Error(),
			}
		}
		return http.StatusNoContent, nil

	case "smf":
		if ctx.SmfSubscriptionId == "" {
			return http.StatusConflict, &models.ProblemDetails{
				Status: http.StatusConflict,
				Title:  "Not Subscribed",
				Detail: "No active SMF subscription to cancel",
			}
		}
		smfCfg := eventSubCfg.SmfEventSubscription
		if smfCfg == nil {
			return http.StatusUnprocessableEntity, &models.ProblemDetails{
				Status: http.StatusUnprocessableEntity,
				Detail: "smfEventSubscription missing from nwdafcfg.yaml",
			}
		}
		logger.SBILog.Infof("[Management] Unsubscribing from SMF events (id=%s)", ctx.SmfSubscriptionId)
		if err := s.Consumer().UnsubscribeSmfEvents(smfCfg.NfUri, ctx.SmfSubscriptionId); err != nil {
			return http.StatusInternalServerError, &models.ProblemDetails{
				Status: http.StatusInternalServerError,
				Title:  "Unsubscription Failed",
				Detail: err.Error(),
			}
		}
		return http.StatusNoContent, nil
	}

	return http.StatusBadRequest, &models.ProblemDetails{
		Status: http.StatusBadRequest,
		Detail: "unknown NF type: " + nfType,
	}
}

// parseOptionalSubscribeRequest reads an optional JSON body. Returns nil if
// the body is empty or absent.
func parseOptionalSubscribeRequest(c *gin.Context) *processor.NfSubscribeRequest {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil || len(body) == 0 {
		return nil
	}
	var req processor.NfSubscribeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}
	return &req
}
