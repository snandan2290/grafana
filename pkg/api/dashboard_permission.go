package api

import (
	"time"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/middleware"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/guardian"
)

func GetDashboardPermissionList(c *middleware.Context) Response {
	dashId := c.ParamsInt64(":dashboardId")

	_, rsp := getDashboardHelper(c.OrgId, "", dashId, "")
	if rsp != nil {
		return rsp
	}

	guardian := guardian.New(dashId, c.OrgId, c.SignedInUser)

	if canAdmin, err := guardian.CanAdmin(); err != nil || !canAdmin {
		return dashboardGuardianResponse(err)
	}

	acl, err := guardian.GetAcl()
	if err != nil {
		return ApiError(500, "Failed to get dashboard permissions", err)
	}

	for _, perm := range acl {
		if perm.Slug != "" {
			perm.Url = m.GetDashboardFolderUrl(perm.IsFolder, perm.Uid, perm.Slug)
		}
	}

	return Json(200, acl)
}

func UpdateDashboardPermissions(c *middleware.Context, apiCmd dtos.UpdateDashboardAclCommand) Response {
	dashId := c.ParamsInt64(":dashboardId")

	_, rsp := getDashboardHelper(c.OrgId, "", dashId, "")
	if rsp != nil {
		return rsp
	}

	guardian := guardian.New(dashId, c.OrgId, c.SignedInUser)
	if canAdmin, err := guardian.CanAdmin(); err != nil || !canAdmin {
		return dashboardGuardianResponse(err)
	}

	cmd := m.UpdateDashboardAclCommand{}
	cmd.DashboardId = dashId

	for _, item := range apiCmd.Items {
		cmd.Items = append(cmd.Items, &m.DashboardAcl{
			OrgId:       c.OrgId,
			DashboardId: dashId,
			UserId:      item.UserId,
			TeamId:      item.TeamId,
			Role:        item.Role,
			Permission:  item.Permission,
			Created:     time.Now(),
			Updated:     time.Now(),
		})
	}

	if okToUpdate, err := guardian.CheckPermissionBeforeUpdate(m.PERMISSION_ADMIN, cmd.Items); err != nil || !okToUpdate {
		if err != nil {
			return ApiError(500, "Error while checking dashboard permissions", err)
		}

		return ApiError(403, "Cannot remove own admin permission for a folder", nil)
	}

	if err := bus.Dispatch(&cmd); err != nil {
		if err == m.ErrDashboardAclInfoMissing || err == m.ErrDashboardPermissionDashboardEmpty {
			return ApiError(409, err.Error(), err)
		}
		return ApiError(500, "Failed to create permission", err)
	}

	return ApiSuccess("Dashboard permissions updated")
}
