package model

type DecoySite struct {
	Id     int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name   string `json:"name" form:"name"`
	Config string `json:"config" form:"config"`
}
