package repo

import (
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/global"
)

type NodeRepo struct{}

type INodeRepo interface {
	Get(opts ...global.DBOption) (model.Node, error)
	List(opts ...global.DBOption) ([]model.Node, error)
	Create(node *model.Node) error
	Update(id uint, vars map[string]interface{}) error
	Delete(opts ...global.DBOption) error
}

func NewINodeRepo() INodeRepo {
	return &NodeRepo{}
}

func (n *NodeRepo) Get(opts ...global.DBOption) (model.Node, error) {
	var node model.Node
	db := global.DB.Model(&model.Node{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.First(&node).Error
	return node, err
}

func (n *NodeRepo) List(opts ...global.DBOption) ([]model.Node, error) {
	var nodes []model.Node
	db := global.DB.Model(&model.Node{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.Find(&nodes).Error
	return nodes, err
}

func (n *NodeRepo) Create(node *model.Node) error {
	return global.DB.Create(node).Error
}

func (n *NodeRepo) Update(id uint, vars map[string]interface{}) error {
	return global.DB.Model(&model.Node{}).Where("id = ?", id).Updates(vars).Error
}

func (n *NodeRepo) Delete(opts ...global.DBOption) error {
	db := global.DB
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.Node{}).Error
}
