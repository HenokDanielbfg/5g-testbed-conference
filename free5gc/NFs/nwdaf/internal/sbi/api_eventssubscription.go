package sbi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/nwdaf/internal/sbi/processor"
	"github.com/free5gc/nwdaf/pkg/factory"
	"github.com/free5gc/openapi/models"
)

// CreateSubscription handles POST /nnwdaf-eventssubscription/v1/subscriptions
func (s *Server) CreateSubscription(c *gin.Context) {
	var sub processor.NwdafEventsSubscription
	if err := c.ShouldBindJSON(&sub); err != nil {
		logger.SBILog.Errorf("CreateSubscription: failed to bind JSON body: %v", err)
		pd := &models.ProblemDetails{
			Title:  "Malformed request syntax",
			Status: http.StatusBadRequest,
			Detail: err.Error(),
		}
		c.JSON(http.StatusBadRequest, pd)
		return
	}

	result, pd := s.Processor().HandleCreateSubscription(&sub)
	if pd != nil {
		c.JSON(int(pd.Status), pd)
		return
	}

	c.Header("Location", factory.NwdafEventsSubscriptionResUriPrefix+"/subscriptions/"+result.SubscriptionId)
	c.JSON(http.StatusCreated, result)
}

// DeleteSubscription handles DELETE /nnwdaf-eventssubscription/v1/subscriptions/:subscriptionId
func (s *Server) DeleteSubscription(c *gin.Context) {
	subscriptionId := c.Param("subscriptionId")

	pd := s.Processor().HandleDeleteSubscription(subscriptionId)
	if pd != nil {
		c.JSON(int(pd.Status), pd)
		return
	}

	c.Status(http.StatusNoContent)
}

// UpdateSubscription handles PUT /nnwdaf-eventssubscription/v1/subscriptions/:subscriptionId
func (s *Server) UpdateSubscription(c *gin.Context) {
	subscriptionId := c.Param("subscriptionId")

	var sub processor.NwdafEventsSubscription
	if err := c.ShouldBindJSON(&sub); err != nil {
		logger.SBILog.Errorf("UpdateSubscription: failed to bind JSON body: %v", err)
		pd := &models.ProblemDetails{
			Title:  "Malformed request syntax",
			Status: http.StatusBadRequest,
			Detail: err.Error(),
		}
		c.JSON(http.StatusBadRequest, pd)
		return
	}

	result, pd := s.Processor().HandleUpdateSubscription(subscriptionId, &sub)
	if pd != nil {
		c.JSON(int(pd.Status), pd)
		return
	}

	c.JSON(http.StatusOK, result)
}
