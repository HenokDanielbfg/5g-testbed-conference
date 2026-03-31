package sbi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/free5gc/nwdaf/internal/logger"
)

// GetAnalyticsInfo handles GET /nnwdaf-analyticsinfo/v1/analytics
func (s *Server) GetAnalyticsInfo(c *gin.Context) {
	analyticsId := c.Query("event-id")
	if analyticsId == "" {
		logger.SBILog.Warn("GetAnalyticsInfo: missing required query parameter 'event-id'")
	}

	queryParams := c.Request.URL.Query()

	data, pd := s.Processor().HandleGetAnalyticsInfo(analyticsId, queryParams)
	if pd != nil {
		c.JSON(int(pd.Status), pd)
		return
	}

	if data != nil {
		c.JSON(http.StatusOK, data)
	} else {
		c.Status(http.StatusNoContent)
	}
}
