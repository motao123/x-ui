package service

import (
	"regexp"
	"strings"
	"time"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/util/common"
	"x-ui/util/random"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProxyUserService struct{}

var proxyUserTokenPattern = regexp.MustCompile(`^[A-Za-z0-9]{24,128}$`)

type ProxyUserPayload struct {
	Id         int    `json:"id" form:"id"`
	Name       string `json:"name" form:"name"`
	Enable     bool   `json:"enable" form:"enable"`
	Token      string `json:"token" form:"token"`
	UUID       string `json:"uuid" form:"uuid"`
	Password   string `json:"password" form:"password"`
	Total      int64  `json:"total" form:"total"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`
	InboundIds []int  `json:"inboundIds" form:"inboundIds"`
}

type ProxyUserView struct {
	model.ProxyUser
	InboundIds []int `json:"inboundIds"`
}

func (s *ProxyUserService) List() ([]*ProxyUserView, error) {
	db := database.GetDB()
	var users []*model.ProxyUser
	if err := db.Order("id desc").Find(&users).Error; err != nil {
		return nil, err
	}
	views := make([]*ProxyUserView, 0, len(users))
	for _, user := range users {
		ids, err := s.GetInboundIds(user.Id)
		if err != nil {
			return nil, err
		}
		views = append(views, &ProxyUserView{ProxyUser: *user, InboundIds: ids})
	}
	return views, nil
}

func (s *ProxyUserService) Get(id int) (*model.ProxyUser, error) {
	user := &model.ProxyUser{}
	err := database.GetDB().First(user, id).Error
	return user, err
}

func (s *ProxyUserService) GetByToken(token string) (*model.ProxyUser, error) {
	user := &model.ProxyUser{}
	err := database.GetDB().Where("token = ?", token).First(user).Error
	return user, err
}

func (s *ProxyUserService) GetInboundIds(proxyUserId int) ([]int, error) {
	var bindings []*model.ProxyUserInbound
	if err := database.GetDB().Where("proxy_user_id = ?", proxyUserId).Find(&bindings).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.InboundId)
	}
	return ids, nil
}

func (s *ProxyUserService) Save(payload *ProxyUserPayload) error {
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		return common.NewError("代理用户名称不能为空")
	}
	now := time.Now().Unix()
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		user := &model.ProxyUser{}
		if payload.Id > 0 {
			if err := tx.First(user, payload.Id).Error; err != nil {
				return err
			}
		} else {
			user.Enable = true
			user.CreatedAt = now
			user.Token = random.Seq(24)
			user.UUID = uuid.NewString()
			user.Password = random.Seq(16)
		}
		user.Name = payload.Name
		user.Enable = payload.Enable
		if payload.Token != "" {
			token := strings.TrimSpace(payload.Token)
			if !proxyUserTokenPattern.MatchString(token) {
				return common.NewError("订阅令牌必须是 24-128 位字母或数字")
			}
			user.Token = token
		}
		if payload.UUID != "" {
			user.UUID = strings.TrimSpace(payload.UUID)
		}
		if payload.Password != "" {
			user.Password = strings.TrimSpace(payload.Password)
		}
		user.Total = payload.Total
		user.ExpiryTime = payload.ExpiryTime
		user.UpdatedAt = now
		if err := tx.Save(user).Error; err != nil {
			return err
		}
		if err := tx.Where("proxy_user_id = ?", user.Id).Delete(&model.ProxyUserInbound{}).Error; err != nil {
			return err
		}
		seen := map[int]bool{}
		for _, inboundId := range payload.InboundIds {
			if inboundId <= 0 || seen[inboundId] {
				continue
			}
			seen[inboundId] = true
			if err := tx.Create(&model.ProxyUserInbound{ProxyUserId: user.Id, InboundId: inboundId}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *ProxyUserService) Delete(id int) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("proxy_user_id = ?", id).Delete(&model.ProxyUserInbound{}).Error; err != nil {
			return err
		}
		if err := tx.Where("proxy_user_id = ?", id).Delete(&model.SubscriptionAccess{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&model.ProxyUser{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *ProxyUserService) GetUsersForInbound(inboundId int) ([]*model.ProxyUser, error) {
	var bindings []*model.ProxyUserInbound
	if err := database.GetDB().Where("inbound_id = ?", inboundId).Find(&bindings).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.ProxyUserId)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var users []*model.ProxyUser
	now := time.Now().UnixMilli()
	err := database.GetDB().Where("id IN ? AND enable = ? AND (expiry_time = 0 OR expiry_time > ?)", ids, true, now).Find(&users).Error
	return users, err
}

func (s *ProxyUserService) RotateToken(id int) (string, error) {
	token := random.Seq(24)
	err := database.GetDB().Model(&model.ProxyUser{}).Where("id = ?", id).Updates(map[string]interface{}{
		"token":      token,
		"updated_at": time.Now().Unix(),
	}).Error
	return token, err
}

func (s *ProxyUserService) ResetTraffic(id int) error {
	return database.GetDB().Model(&model.ProxyUser{}).Where("id = ?", id).Updates(map[string]interface{}{
		"up":         0,
		"down":       0,
		"updated_at": time.Now().Unix(),
	}).Error
}
