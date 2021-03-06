// Copyright 2014 The dbrouter Author. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dbrouter

import (
	"fmt"
	"github.com/bitly/go-simplejson"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/shawnfeng/sutil/slog"
	"github.com/shawnfeng/sutil/stime"
	"time"
)

type dbSql struct {
	dbType   string
	dbName   string
	dbAddrs  string
	timeOut  time.Duration
	userName string
	passWord string
	db       *sqlx.DB
}

func (m *dbSql) getType() string {
	return m.dbType
}

func NewdbSql(dbtype, dbname string, cfg []byte) (*dbSql, error) {

	cfg_json, err := simplejson.NewJson(cfg)
	if err != nil {
		return nil, fmt.Errorf("instance db:%s type:%s config:%s unmarshal err:%s", dbname, dbtype, cfg, err)
	}

	addrs, err := cfg_json.Get("addrs").StringArray()
	if err != nil {
		return nil, fmt.Errorf("instance db:%s type:%s config:%s addrs err:%s", dbname, dbtype, cfg, err)
	}

	if len(addrs) != 1 {
		return nil, fmt.Errorf("instance db:%s type:%s config:%s len(addrs)!=1", dbname, dbtype, cfg)
	}

	timeout := 60 * time.Second
	if t, err := cfg_json.Get("timeout").Int64(); err == nil {
		timeout = time.Duration(t) * time.Millisecond
	}

	user, _ := cfg_json.Get("user").String()
	passwd, _ := cfg_json.Get("passwd").String()

	info := &dbSql{
		dbType:   dbtype,
		dbName:   dbname,
		dbAddrs:  addrs[0],
		timeOut:  timeout,
		userName: user,
		passWord: passwd,
	}

	info.db, err = dial(info)
	info.db.SetMaxIdleConns(8)
	return info, err
}

func dial(info *dbSql) (db *sqlx.DB, err error) {

	var dataSourceName string
	if info.dbType == DB_TYPE_MYSQL {
		dataSourceName = fmt.Sprintf("%s:%s@tcp(%s)/%s", info.userName, info.passWord, info.dbAddrs, info.dbName)

	} else if info.dbType == DB_TYPE_POSTGRES {
		dataSourceName = fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
			info.userName, info.passWord, info.dbAddrs, info.dbName)
	}

	return sqlx.Connect(info.dbType, dataSourceName)
}

func (m *dbSql) getDB() *sqlx.DB {
	return m.db
}

func (m *Router) SqlExec(cluster, table string, query func(*sqlx.DB) error) error {
	st := stime.NewTimeStat()

	ins_name := m.dbCls.getInstance(cluster, table)
	if ins_name == "" {
		return fmt.Errorf("cluster instance not find: cluster:%s table:%s", cluster, table)
	}

	durInsn := st.Duration()
	st.Reset()

	ins := m.dbIns.get(ins_name)
	if ins == nil {
		return fmt.Errorf("db instance not find: cluster:%s table:%s", cluster, table)
	}

	durIns := st.Duration()
	st.Reset()

	dbsql, ok := ins.(*dbSql)
	if !ok {
		return fmt.Errorf("db instance type error: cluster:%s table:%s type:%s", cluster, table, ins.getType())
	}

	durInst := st.Duration()
	st.Reset()

	db := dbsql.getDB()

	defer func() {
		dur := st.Duration()
		slog.Infof("[SQL] cls:%s table:%s nmins:%d ins:%d rins:%d query:%d", cluster, table, durInsn, durIns, durInst, dur)
	}()

	return query(db)
}
