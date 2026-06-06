package main

import (
	"os"
	"path/filepath"
	"text/template"

	"github.com/BurntSushi/toml"
)

type serverConfig struct {
	Debug           bool     `toml:"debug"`
	ListenerType    string   `toml:"listenerType"`
	Listen          string   `toml:"listen"`
	BodyLimit       int      `toml:"bodyLimit"`
	ReadTimeout     int      `toml:"readTimeout"`
	WriteTimeout    int      `toml:"writeTimeout"`
	IdleTimeout     int      `toml:"idleTimeout"`
	ReadBufferSize  int      `toml:"readBufferSize"`
	WriteBufferSize int      `toml:"writeBufferSize"`
	TrustedProxies  []string `toml:"trustedProxies"`
}

type peerApiCenterConfig struct {
	APIURL                      string   `toml:"apiUrl"`
	ProbeServerIPv4             string   `toml:"probeServerIPv4"`
	ProbeServerIPv6             string   `toml:"probeServerIPv6"`
	ProbeServerIPv6Prefix       string   `toml:"probeServerIPv6Prefix"`
	ProbeServerPort             int      `toml:"probeServerPort"`
	Secret                      string   `toml:"secret"`
	RequestTimeout              int      `toml:"requestTimeout"`
	RouterUUID                  string   `toml:"routerUuid"`
	AgentSecret                 string   `toml:"agentSecret"`
	HeartbeatInterval           int      `toml:"heartbeatInterval"`
	SyncInterval                int      `toml:"syncInterval"`
	MetricInterval              int      `toml:"metricInterval"`
	WanInterfaces               []string `toml:"wanInterfaces"`
	SessionPassthroughJwtSecert string   `toml:"sessionPassthroughJwtSecert"`
	InterfaceIpAllowPublic      bool     `toml:"interfaceIpAllowPublic"`
	InterfaceIpBlacklist        []string `toml:"interfaceIpBlacklist"`
}

type birdConfig struct {
	ControlSocket           string             `toml:"controlSocket"`
	PoolSize                int                `toml:"poolSize"`
	PoolSizeMax             int                `toml:"poolSizeMax"`
	ConnectionMaxRetries    int                `toml:"connectionMaxRetries"`
	ConnectionRetryDelayMs  int                `toml:"connectionRetryDelayMs"`
	BGPPeerConfDir          string             `toml:"bgpPeerConfDir"`
	BGPPeerConfTemplateFile string             `toml:"bgpPeerConfTemplateFile"`
	BGPPeerConfTemplate     *template.Template `toml:"-"`
	IPCommandPath           string             `toml:"ipCommandPath"`
}

type wireGuardConfig struct {
	WGCommandPath                  string `toml:"wgCommandPath"`
	LocalEndpointHost              string `toml:"localEndpointHost"`
	PrivateKeyPath                 string `toml:"privateKeyPath"`
	PublicKeyPath                  string `toml:"publicKeyPath"`
	PrivateKey                     string `toml:"-"`
	PublicKey                      string `toml:"-"`
	PersistentKeepaliveInterval    int    `toml:"persistentKeepaliveInterval"`
	AllowedIPs                     string `toml:"allowedIps"`
	DNSUpdateInterval              int    `toml:"dnsUpdateInterval"`
	DN42BandwidthCommunity         int    `toml:"dn42BandwidthCommunity"`
	DN42InterfaceSecurityCommunity int    `toml:"dn42InterfaceSecurityCommunity"`
}

type greConfig struct {
	LocalEndpointHost4             string `toml:"localEndpointHost4"`
	LocalEndpointHost6             string `toml:"localEndpointHost6"`
	LocalEndpointDesc4             string `toml:"localEndpointDesc4"`
	LocalEndpointDesc6             string `toml:"localEndpointDesc6"`
	DN42BandwidthCommunity         int    `toml:"dn42BandwidthCommunity"`
	DN42InterfaceSecurityCommunity int    `toml:"dn42InterfaceSecurityCommunity"`
}

type loggerConfig struct {
	File           string `toml:"file"`
	MaxSize        int    `toml:"maxSize"`
	MaxBackups     int    `toml:"maxBackups"`
	MaxAge         int    `toml:"maxAge"`
	Compress       bool   `toml:"compress"`
	ConsoleLogging bool   `toml:"consoleLogging"`
}

type metricConfig struct {
	AutoTeardown                  bool     `toml:"autoTeardown"`
	MaxMindGeoLiteCountryMmdbPath string   `toml:"maxMindGeoLiteCountryMmdbPath"`
	GeoIPCountryMode              string   `toml:"geoIpCountryMode"`
	BlacklistGeoCountries         []string `toml:"blacklistGeoCountries"`
	WhitelistGeoCountries         []string `toml:"whitelistGeoCountries"`
	PingCommandPath               string   `toml:"pingCommandPath"`
	PingTimeout                   int      `toml:"pingTimeout"`
	PingCount                     int      `toml:"pingCount"`
	PingCountOnFail               int      `toml:"pingCountOnFail"`
	PingWorkerCount               int      `toml:"pingWorkerCount"`
	SessionWorkerCount            int      `toml:"sessionWorkerCount"`
	MaxRTTMetricsHistroy          int      `toml:"maxRTTMetricsHistroy"`
	GeoCheckInterval              int      `toml:"geoCheckInterval"`
	FilterParamsUpdateInterval    int      `toml:"filterParamsUpdateInterval"`
}

type sysctlConfig struct {
	CommandPath        string `toml:"commandPath"`
	IfaceIPForwarding  bool   `toml:"ifaceIpForwarding"`
	IfaceIP6Forwarding bool   `toml:"ifaceIp6Forwarding"`
	IfaceIP6AcceptRA   bool   `toml:"ifaceIp6AcceptRa"`
	IfaceIP6AutoConfig bool   `toml:"ifaceIp6AutoConfig"`
	IfaceRPFilter      int    `toml:"ifaceRpFilter"`
	IfaceAcceptLocal   bool   `toml:"ifaceAcceptLocal"`
}

type peerProbeConfig struct {
	Enabled                     bool   `toml:"enabled"`
	IntervalSeconds             int    `toml:"intervalSeconds"`
	ProbePacketCount            int    `toml:"probePacketCount"`
	ProbePacketIntervalMs       int    `toml:"probePacketIntervalMs"`
	ProbePacketEncryptionKey    string `toml:"probePacketEncryptionKey"`
	SessionWorkerCount          int    `toml:"sessionWorkerCount"`
	ProbePacketBanner           string `toml:"probePacketBanner"`
	ProbeSummaryCooldownSeconds int    `toml:"probeSummaryCooldownSeconds"`
}

type ipConfig struct {
	IPv4          string `toml:"ipv4"`
	IPv6          string `toml:"ipv6"`
	IPv6LinkLocal string `toml:"ipv6LinkLocal"`
}

type config struct {
	Server    serverConfig        `toml:"server"`
	PeerAPI   peerApiCenterConfig `toml:"peerApiCenter"`
	IP        ipConfig            `toml:"ipConfig"`
	Bird      birdConfig          `toml:"bird"`
	Sysctl    sysctlConfig        `toml:"sysctl"`
	Metric    metricConfig        `toml:"metric"`
	WireGuard wireGuardConfig     `toml:"wireguard"`
	GRE       greConfig           `toml:"gre"`
	Logger    loggerConfig        `toml:"logger"`
	PeerProbe peerProbeConfig     `toml:"peerProbe"`
}

func loadConfig(filename string) (*config, error) {
	cfg := &config{}

	_, err := toml.DecodeFile(filename, cfg)
	if err != nil {
		return nil, err
	}

	// Optional overlay files
	base := filepath.Dir(filename)
	overlays := []struct {
		file string
		dest interface{}
	}{
		{"server.toml", &cfg.Server},
		{"bird.toml", &cfg.Bird},
		{"sysctl.toml", &cfg.Sysctl},
	}

	for _, ov := range overlays {
		ovPath := filepath.Join(base, ov.file)
		if _, err := os.Stat(ovPath); os.IsNotExist(err) {
			continue
		}
		var wrapper config
		_, err := toml.DecodeFile(ovPath, &wrapper)
		if err != nil {
			return nil, err
		}
		switch d := ov.dest.(type) {
		case *serverConfig:
			*d = wrapper.Server
		case *birdConfig:
			*d = wrapper.Bird
		case *sysctlConfig:
			*d = wrapper.Sysctl
		}
	}

	if cfg.WireGuard.PrivateKeyPath != "" {
		key, err := os.ReadFile(cfg.WireGuard.PrivateKeyPath)
		if err != nil {
			return cfg, err
		}
		cfg.WireGuard.PrivateKey = string(key)
	}

	if cfg.WireGuard.PublicKeyPath != "" {
		key, err := os.ReadFile(cfg.WireGuard.PublicKeyPath)
		if err != nil {
			return cfg, err
		}
		cfg.WireGuard.PublicKey = string(key)
	}

	if cfg.Bird.BGPPeerConfTemplateFile != "" {
		tmpl, err := template.ParseFiles(cfg.Bird.BGPPeerConfTemplateFile)
		if err != nil {
			return cfg, err
		}
		cfg.Bird.BGPPeerConfTemplate = tmpl
	}

	return cfg, nil
}
