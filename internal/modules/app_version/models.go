package appversion

import "time"

type AppVersionInfo struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppOS       string    `gorm:"column:app_os;not null;unique" json:"app_os"`
	MinBuild    int64     `gorm:"column:min_build;not null" json:"min_build"`
	LatestBuild int64     `gorm:"column:latest_build;not null" json:"latest_build"`
	StoreURL    string    `gorm:"column:store_url;not null" json:"store_url"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (AppVersionInfo) TableName() string { return "wallet_app_version_infos" }
