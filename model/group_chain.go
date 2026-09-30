package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	// GroupChainMaxGroups limits the size of a chain so a malformed request
	// cannot create an unbounded retry path.
	GroupChainMaxGroups = 20
	// GroupChainMaxPerUser matches the reference product's visible 1/10 limit.
	GroupChainMaxPerUser = 10
)

// GroupChain is a reusable, ordered fallback policy for a user's API keys.
// Groups stores stable group names in order. Token binding is deliberately
// separate from this model so editing a chain changes all bound tokens without
// rewriting their rows.
type GroupChain struct {
	Id          int            `json:"id"`
	UserId      int            `json:"user_id" gorm:"index;not null"`
	Name        string         `json:"name" gorm:"size:100;not null"`
	Groups      JSONValue      `json:"groups" gorm:"type:json;not null"`
	CreatedTime int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// NormalizeGroupChainGroups validates and canonicalizes a chain's ordered
// group names. It intentionally rejects "auto": a chain is itself an
// explicit alternative to the existing automatic mode.
func NormalizeGroupChainGroups(groups []string) ([]string, error) {
	if len(groups) == 0 {
		return nil, errors.New("分组链至少需要一个分组")
	}
	if len(groups) > GroupChainMaxGroups {
		return nil, fmt.Errorf("分组链最多包含 %d 个分组", GroupChainMaxGroups)
	}
	normalized := make([]string, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			return nil, errors.New("分组名称不能为空")
		}
		if group == "auto" {
			return nil, errors.New("分组链不能包含 auto 分组")
		}
		if _, ok := seen[group]; ok {
			return nil, fmt.Errorf("分组链包含重复分组 %q", group)
		}
		seen[group] = struct{}{}
		normalized = append(normalized, group)
	}
	return normalized, nil
}

func (chain *GroupChain) GetGroups() ([]string, error) {
	if chain == nil || len(chain.Groups) == 0 {
		return nil, nil
	}
	var groups []string
	if err := common.Unmarshal(chain.Groups, &groups); err != nil {
		return nil, err
	}
	return NormalizeGroupChainGroups(groups)
}

func (chain *GroupChain) SetGroups(groups []string) error {
	normalized, err := NormalizeGroupChainGroups(groups)
	if err != nil {
		return err
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return err
	}
	chain.Groups = JSONValue(data)
	return nil
}

func IsGroupChainNameDuplicated(userID, id int, name string) (bool, error) {
	var count int64
	err := DB.Model(&GroupChain{}).
		Where("user_id = ? AND name = ? AND id <> ?", userID, name, id).
		Count(&count).Error
	return count > 0, err
}

func CountUserGroupChains(userID int) (int64, error) {
	var count int64
	err := DB.Model(&GroupChain{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func GetUserGroupChains(userID int) ([]*GroupChain, error) {
	var chains []*GroupChain
	err := DB.Where("user_id = ?", userID).Order("id ASC").Find(&chains).Error
	return chains, err
}

func GetUserGroupChain(id, userID int) (*GroupChain, error) {
	var chain GroupChain
	err := DB.Where("id = ? AND user_id = ?", id, userID).First(&chain).Error
	return &chain, err
}

func (chain *GroupChain) Insert() error {
	now := common.GetTimestamp()
	chain.CreatedTime = now
	chain.UpdatedTime = now
	return DB.Create(chain).Error
}

func (chain *GroupChain) Update() error {
	chain.UpdatedTime = common.GetTimestamp()
	return DB.Save(chain).Error
}

func DeleteUserGroupChain(id, userID int) error {
	return DB.Where("id = ? AND user_id = ?", id, userID).Delete(&GroupChain{}).Error
}
