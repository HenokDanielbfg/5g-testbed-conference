package sbi

import "github.com/gin-gonic/gin"

type Route struct {
	Method      string
	Pattern     string
	HandlerFunc gin.HandlerFunc
}

func applyRoutes(group *gin.RouterGroup, routes []Route) {
	for _, route := range routes {
		switch route.Method {
		case "GET":
			group.GET(route.Pattern, route.HandlerFunc)
		case "POST":
			group.POST(route.Pattern, route.HandlerFunc)
		case "PUT":
			group.PUT(route.Pattern, route.HandlerFunc)
		case "DELETE":
			group.DELETE(route.Pattern, route.HandlerFunc)
		case "PATCH":
			group.PATCH(route.Pattern, route.HandlerFunc)
		}
	}
}

func (s *Server) getAnalyticsInfoRoutes() []Route {
	return []Route{
		{
			Method:      "GET",
			Pattern:     "/analytics",
			HandlerFunc: s.GetAnalyticsInfo,
		},
	}
}

func (s *Server) getEventsSubscriptionRoutes() []Route {
	return []Route{
		{
			Method:      "POST",
			Pattern:     "/subscriptions",
			HandlerFunc: s.CreateSubscription,
		},
		{
			Method:      "DELETE",
			Pattern:     "/subscriptions/:subscriptionId",
			HandlerFunc: s.DeleteSubscription,
		},
		{
			Method:      "PUT",
			Pattern:     "/subscriptions/:subscriptionId",
			HandlerFunc: s.UpdateSubscription,
		},
	}
}

func (s *Server) getManagementRoutes() []Route {
	return []Route{
		{
			Method:      "GET",
			Pattern:     "/nf-subscriptions",
			HandlerFunc: s.GetNfSubscriptions,
		},
		{
			Method:      "POST",
			Pattern:     "/nf-subscriptions/amf",
			HandlerFunc: s.SubscribeAmfEvents,
		},
		{
			Method:      "DELETE",
			Pattern:     "/nf-subscriptions/amf",
			HandlerFunc: s.UnsubscribeAmfEvents,
		},
		{
			Method:      "POST",
			Pattern:     "/nf-subscriptions/smf",
			HandlerFunc: s.SubscribeSmfEvents,
		},
		{
			Method:      "DELETE",
			Pattern:     "/nf-subscriptions/smf",
			HandlerFunc: s.UnsubscribeSmfEvents,
		},
	}
}

func (s *Server) getCallbackRoutes() []Route {
	return []Route{
		{
			Method:      "POST",
			Pattern:     "/amf-event-notify",
			HandlerFunc: s.AmfEventNotifyCallback,
		},
		{
			Method:      "POST",
			Pattern:     "/smf-event-notify",
			HandlerFunc: s.SmfEventNotifyCallback,
		},
	}
}
