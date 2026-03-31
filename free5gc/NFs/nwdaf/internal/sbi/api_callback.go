package sbi

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/openapi/models"
)

func (s *Server) AmfEventNotifyCallback(c *gin.Context) {
	logger.CallbackLog.Infof("Received AMF event notification")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.CallbackLog.Errorf("Failed to read AMF notification body: %v", err)
		c.JSON(http.StatusBadRequest, models.ProblemDetails{
			Status: http.StatusBadRequest,
			Title:  "Bad Request",
			Detail: "Failed to read request body",
		})
		return
	}

	var notification models.AmfEventNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		logger.CallbackLog.Errorf("Failed to unmarshal AMF notification: %v", err)
		c.JSON(http.StatusBadRequest, models.ProblemDetails{
			Status: http.StatusBadRequest,
			Title:  "Bad Request",
			Detail: "Invalid AMF event notification format",
		})
		return
	}

	s.Processor().HandleAmfEventNotification(&notification)
	c.Status(http.StatusNoContent)
}

func (s *Server) SmfEventNotifyCallback(c *gin.Context) {
	logger.CallbackLog.Infof("Received SMF event notification")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.CallbackLog.Errorf("Failed to read SMF notification body: %v", err)
		c.JSON(http.StatusBadRequest, models.ProblemDetails{
			Status: http.StatusBadRequest,
			Title:  "Bad Request",
			Detail: "Failed to read request body",
		})
		return
	}

	var notification models.NsmfEventExposureNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		logger.CallbackLog.Errorf("Failed to unmarshal SMF notification: %v", err)
		c.JSON(http.StatusBadRequest, models.ProblemDetails{
			Status: http.StatusBadRequest,
			Title:  "Bad Request",
			Detail: "Invalid SMF event notification format",
		})
		return
	}

	s.Processor().HandleSmfEventNotification(&notification)
	c.Status(http.StatusNoContent)
}
