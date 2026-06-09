package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AccountInspectionHandler struct {
	svc *service.AccountInspectionService
}

func NewAccountInspectionHandler(svc *service.AccountInspectionService) *AccountInspectionHandler {
	return &AccountInspectionHandler{svc: svc}
}

func (h *AccountInspectionHandler) GetStatus(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Error(c, http.StatusServiceUnavailable, "account inspection service unavailable")
		return
	}
	status, err := h.svc.GetStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *AccountInspectionHandler) UpdateSettings(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Error(c, http.StatusServiceUnavailable, "account inspection service unavailable")
		return
	}
	var req service.AccountInspectionSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	settings, err := h.svc.UpdateSettings(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *AccountInspectionHandler) Start(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Error(c, http.StatusServiceUnavailable, "account inspection service unavailable")
		return
	}
	run, err := h.svc.StartRun(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Accepted(c, run)
}

func (h *AccountInspectionHandler) Stop(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Error(c, http.StatusServiceUnavailable, "account inspection service unavailable")
		return
	}
	run, err := h.svc.StopRun(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, run)
}
