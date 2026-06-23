package model

type Protocol string

const (
	VMess       Protocol = "vmess"
	VLESS       Protocol = "vless"
	Dokodemo    Protocol = "dokodemo-door"
	Http        Protocol = "http"
	Trojan      Protocol = "trojan"
	Shadowsocks Protocol = "shadowsocks"
	Socks       Protocol = "socks"
)

type User struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (User) TableName() string { return "users" }

type Inbound struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	UserId     int    `json:"-"`
	Up         int64  `json:"up" form:"up"`
	Down       int64  `json:"down" form:"down"`
	Total      int64  `json:"total" form:"total"`
	Remark     string `json:"remark" form:"remark"`
	Enable     bool   `json:"enable" form:"enable"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`

	// config part
	Listen         string   `json:"listen" form:"listen"`
	Port           int      `json:"port" form:"port" gorm:"unique"`
	Protocol       Protocol `json:"protocol" form:"protocol"`
	Settings       string   `json:"settings" form:"settings"`
	StreamSettings string   `json:"streamSettings" form:"streamSettings"`
	Tag            string   `json:"tag" form:"tag" gorm:"unique"`
	Sniffing       string   `json:"sniffing" form:"sniffing"`
}

func (Inbound) TableName() string { return "inbounds" }

type Setting struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

func (Setting) TableName() string { return "settings" }

// TrafficHistory 记录周期性的总流量快照，用于趋势图展示。
type TrafficHistory struct {
	Id       int   `json:"id" gorm:"primaryKey;autoIncrement"`
	Up       int64 `json:"up"`
	Down     int64 `json:"down"`
	RecordAt int64 `json:"recordAt" gorm:"index"`
}

func (TrafficHistory) TableName() string { return "traffic_histories" }

type ProxyUser struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name       string `json:"name" form:"name" gorm:"not null"`
	Enable     bool   `json:"enable" form:"enable"`
	Token      string `json:"token" form:"token" gorm:"uniqueIndex;not null"`
	UUID       string `json:"uuid" form:"uuid"`
	Password   string `json:"password" form:"password"`
	Up         int64  `json:"up" form:"up"`
	Down       int64  `json:"down" form:"down"`
	Total      int64  `json:"total" form:"total"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

func (ProxyUser) TableName() string { return "proxy_users" }

type ProxyUserInbound struct {
	Id          int `json:"id" gorm:"primaryKey;autoIncrement"`
	ProxyUserId int `json:"proxyUserId" gorm:"index;not null"`
	InboundId   int `json:"inboundId" gorm:"index;not null"`
}

func (ProxyUserInbound) TableName() string { return "proxy_user_inbounds" }

type SubscriptionAccess struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ProxyUserId int    `json:"proxyUserId" gorm:"index;not null"`
	Format      string `json:"format"`
	UserAgent   string `json:"userAgent"`
	RemoteIp    string `json:"remoteIp"`
	AccessedAt  int64  `json:"accessedAt" gorm:"index"`
}

func (SubscriptionAccess) TableName() string { return "subscription_accesses" }
