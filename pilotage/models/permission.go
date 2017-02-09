/*
Copyright 2014 Huawei Technologies Co., Ltd. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

const (
	RESOURCE_COMPONENT = "component"
	RESOURCE_WORKFLOW  = "workflow"
	RESOURCE_SETTING   = "setting"
)

type Permission struct {
	ID        int64      `json:"id" gorm:"primary_key"` //
	Role      string     `json:"-"`                     //
	Resource  string     `json:"-"`                     //
	Operation string     `json:"-"`                     //
	CreatedAt time.Time  `json:"created" sql:""`        //
	UpdatedAt time.Time  `json:"updated" sql:""`        //
	DeletedAt *time.Time `json:"deleted" sql:"index"`   //
}

//TableName is return the table name of Permission in MySQL database.
func (e *Permission) TableName() string {
	return "permission"
}

func (e *Permission) GetPermission() *gorm.DB {
	return db.Model(&Permission{})
}
