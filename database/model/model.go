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

type Certificate struct {
	Id        int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name      string `json:"name" form:"name" gorm:"not null"`
	Domain    string `json:"domain" form:"domain"`
	CertFile  string `json:"certFile" form:"certFile" gorm:"not null"`
	KeyFile   string `json:"keyFile" form:"keyFile" gorm:"not null"`
	Source    string `json:"source" form:"source"`
	Mode      string `json:"mode" form:"mode"`
	AcmeId    int    `json:"acmeId" form:"acmeId" gorm:"index"`
	DnsId     int    `json:"dnsId" form:"dnsId" gorm:"index"`
	AutoRenew bool   `json:"autoRenew" form:"autoRenew"`
	NotBefore int64  `json:"notBefore"`
	NotAfter  int64  `json:"notAfter"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

func (Certificate) TableName() string { return "certificates" }

type AcmeAccount struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name       string `json:"name" form:"name" gorm:"not null"`
	Email      string `json:"email" form:"email" gorm:"not null"`
	Provider   string `json:"provider" form:"provider"`
	PrivateKey string `json:"-" form:"privateKey"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

func (AcmeAccount) TableName() string { return "acme_accounts" }

type DnsAccount struct {
	Id        int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name      string `json:"name" form:"name" gorm:"not null"`
	Provider  string `json:"provider" form:"provider" gorm:"not null"`
	Key       string `json:"key" form:"key"`
	Secret    string `json:"secret" form:"secret"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

func (DnsAccount) TableName() string { return "dns_accounts" }

type RouteRule struct {
	Id          int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Enable      bool   `json:"enable" form:"enable"`
	Name        string `json:"name" form:"name" gorm:"not null"`
	Domain      string `json:"domain" form:"domain"`
	Ip          string `json:"ip" form:"ip"`
	Protocol    string `json:"protocol" form:"protocol"`
	InboundTag  string `json:"inboundTag" form:"inboundTag"`
	OutboundTag string `json:"outboundTag" form:"outboundTag" gorm:"not null"`
	Sort        int    `json:"sort" form:"sort" gorm:"index"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

func (RouteRule) TableName() string { return "route_rules" }

type Endpoint struct {
	Id        int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Enable    bool   `json:"enable" form:"enable"`
	Name      string `json:"name" form:"name" gorm:"not null"`
	Type      string `json:"type" form:"type" gorm:"not null"`
	Tag       string `json:"tag" form:"tag" gorm:"uniqueIndex;not null"`
	Address   string `json:"address" form:"address"`
	Endpoint  string `json:"endpoint" form:"endpoint"`
	Port      int    `json:"port" form:"port"`
	SecretKey string `json:"secretKey" form:"secretKey"`
	PublicKey string `json:"publicKey" form:"publicKey"`
	Reserved  string `json:"reserved" form:"reserved"`
	Mtu       int    `json:"mtu" form:"mtu"`
	Settings  string `json:"settings" form:"settings"`
	Sort      int    `json:"sort" form:"sort" gorm:"index"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

func (Endpoint) TableName() string { return "endpoints" }

type WarpAccount struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	AccessToken string `json:"-"`
	DeviceId    string `json:"deviceId"`
	LicenseKey  string `json:"licenseKey"`
	PublicKey   string `json:"publicKey"`
	PrivateKey  string `json:"-"`
	AutoUpdate  int    `json:"autoUpdate"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

func (WarpAccount) TableName() string { return "warp_accounts" }
