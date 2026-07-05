package v2

import (
	"github.com/1Panel-dev/1Panel/core/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/core/app/dto"
	"github.com/gin-gonic/gin"
)

func (b *BaseApi) ListNodes(c *gin.Context) {
	var req dto.NodeListReq
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	list, err := nodeService.List(req)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, list)
}

func (b *BaseApi) ListSimpleNodes(c *gin.Context) {
	list, err := nodeService.ListSimple()
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, list)
}

func (b *BaseApi) NodeDashboard(c *gin.Context) {
	dash, err := nodeService.Dashboard()
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, dash)
}

func (b *BaseApi) CheckNode(c *gin.Context) {
	var req dto.NodeCreate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	item, err := nodeService.Check(req)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, item)
}

func (b *BaseApi) CreateNode(c *gin.Context) {
	var req dto.NodeCreate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := nodeService.Create(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

func (b *BaseApi) UpdateNode(c *gin.Context) {
	var req dto.NodeUpdate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := nodeService.Update(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

func (b *BaseApi) DeleteNode(c *gin.Context) {
	var req dto.NodeDelete
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := nodeService.Delete(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

func (b *BaseApi) FavoriteNode(c *gin.Context) {
	var req dto.NodeFavorite
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := nodeService.Favorite(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

func (b *BaseApi) SyncNodes(c *gin.Context) {
	var req dto.NodeBatchOperate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := nodeService.Sync(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

func (b *BaseApi) UpgradeNodes(c *gin.Context) {
	var req dto.NodeBatchOperate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := nodeService.Upgrade(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

func (b *BaseApi) InstallAppToNodes(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.BadRequest(c, err)
		return
	}
	if err := nodeService.InstallAppToNodes(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}
