// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package config

import (
	"context"
	"fmt"

	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/pquerna/ffjson/ffjson"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/job"
	"github.com/goodrain/rainbond/pkg/node/core/store"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
)

//DataCenterConfig 数据中心配置
type DataCenterConfig struct {
	config  *model.GlobalConfig
	options *option.Conf
	ctx     context.Context
	cancel  context.CancelFunc
}

//CreateDataCenterConfig 创建
func CreateDataCenterConfig(options *option.Conf) *DataCenterConfig {
	ctx, cancel := context.WithCancel(context.Background())
	return &DataCenterConfig{
		options: options,
		ctx:     ctx,
		cancel:  cancel,
		config: &model.GlobalConfig{
			Configs: make(map[string]*model.ConfigUnit),
		},
	}
}

//Start 启动，监听配置变化
func (d *DataCenterConfig) Start() {
	res, err := store.DefalutClient.Get(d.options.ConfigStoragePath+"/global", client.WithPrefix())
	if err != nil {
		logrus.Error("load datacenter config error.", err.Error())
	}
	for _, kv := range res.Kvs {
		d.PutConfigKV(kv)
	}
	go func() {
		logrus.Info("datacenter config listener start")
		ch := store.DefalutClient.Watch(d.options.ConfigStoragePath+"/global", client.WithPrefix())
		for {
			select {
			case <-d.ctx.Done():
				return
			case event := <-ch:
				for _, e := range event.Events {
					switch {
					case e.IsCreate(), e.IsModify():
						d.PutConfigKV(e.Kv)
					case e.Type == client.EventTypeDelete:
						d.DeleteConfig(job.GetIDFromKey(string(e.Kv.Key)))
					}
				}
			}
		}
	}()
}

//Stop 停止监听
func (d *DataCenterConfig) Stop() {
	d.cancel()
	logrus.Info("datacenter config listener stop")
}

//GetDataCenterConfig 获取配置
func (d *DataCenterConfig) GetDataCenterConfig() (*model.GlobalConfig, error) {
	if len(d.config.Configs) < 1 {
		dgc := model.CreateDefaultGlobalConfig()
		err := d.PutDataCenterConfig(dgc)
		if err != nil {
			logrus.Error("put datacenter config error,", err.Error())
			return nil, err
		}
		d.config = dgc
	}
	return d.config, nil
}

//PutDataCenterConfig 更改配置
func (d *DataCenterConfig) PutDataCenterConfig(c *model.GlobalConfig) (err error) {
	if c == nil {
		return
	}
	for k, v := range c.Configs {
		d.config.Add(*v)
		_, err = store.DefalutClient.Put(d.options.ConfigStoragePath+"/global/"+k, v.String())
	}
	return err
}

//GetConfig 获取全局配置
func (d *DataCenterConfig) GetConfig(name string) model.ConfigUnit {
	return d.config.Get(name)
}

//PutConfig 增加or更新配置
func (d *DataCenterConfig) PutConfig(c *model.ConfigUnit) error {
	if c.Name == "" {
		return fmt.Errorf("config name can not be empty")
	}
	d.config.Add(*c)
	//持久化
	_, err := store.DefalutClient.Put(d.options.ConfigStoragePath+"/global/"+c.Name, c.String())
	if err != nil {
		logrus.Error("put datacenter config to etcd error.", err.Error())
		return err
	}
	return nil
}

//PutConfigKV 更新
func (d *DataCenterConfig) PutConfigKV(kv *mvccpb.KeyValue) {
	var cn model.ConfigUnit
	if err := ffjson.Unmarshal(kv.Value, &cn); err == nil {
		d.PutConfig(&cn)
	} else {
		logrus.Errorf("parse config error,%s", err.Error())
	}
}

//DeleteConfig 删除配置
func (d *DataCenterConfig) DeleteConfig(name string) {
	d.config.Delete(name)
}