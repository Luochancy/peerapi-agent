// *******************************************************************
// src/config.go
//
// Copyright (C) 2024 iEdon
// Copyright (C) 2026 Luochancy
//
// This file is part of a project derived from peerapi-agent.
// Modified by Luochancy on 2026-06.
//
// Licensed under the GNU General Public License v3.0.
// See the LICENSE file in the project root for details.
// *******************************************************************

package main

import (
	"fmt"
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

// ensureConfig checks config state and generates defaults if needed.
//   - config/ missing: create dir, write all .default files, exit.
//   - config/ exists but config.toml missing: error, refuse to start.
//   - config.toml exists: proceed normally.
func ensureConfig(path string) error {
	dir := filepath.Dir(path)

	_, err := os.Stat(dir)
	if err == nil {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Check if user forgot to rename .default
			defaultPath := path + ".default"
			if _, err := os.Stat(defaultPath); err == nil {
				return fmt.Errorf("%s not found but %s exists. Rename %s to %s and edit it",
					path, defaultPath, defaultPath, path)
			}
			return fmt.Errorf("%s not found. Copy %s.default to %s and edit it",
				path, path, path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	defaultPath := path + ".default"
	if err := writeDefaultConfig(defaultPath); err != nil {
		return err
	}

	// Write overlay template files
	templates := map[string]string{
		"server.toml.default": `# Optional override for HTTP server settings. Remove .default suffix to enable.

[server]
debug = false
listenerType = "tcp"
listen = ":8080"
readTimeout = 30
writeTimeout = 30
idleTimeout = 120
writeBufferSize = 8192
readBufferSize = 8192
bodyLimit = 1048576
trustedProxies = ["127.0.0.1", "::1"]
`,
		"bird.toml.default": `# Optional override for BIRD control and peer template settings. Remove .default suffix to enable.

[bird]
controlSocket = "/var/run/bird/bird.ctl"
poolSize = 5
poolSizeMax = 64
connectionMaxRetries = 5
connectionRetryDelayMs = 50
bgpPeerConfDir = "/etc/bird/peers"
bgpPeerConfTemplateFile = "./templates/peer.conf"
ipCommandPath = "/usr/sbin/ip"
`,
		"sysctl.toml.default": `# Optional override for sysctl settings on new session interfaces. Remove .default suffix to enable.

[sysctl]
commandPath = "/usr/sbin/sysctl"
ifaceIpForwarding = true
ifaceIp6Forwarding = true
ifaceIp6AcceptRa = false
ifaceIp6AutoConfig = false
ifaceRpFilter = 0
ifaceAcceptLocal = true
`,
	}

	for name, content := range templates {
		tplPath := filepath.Join(dir, name)
		if err := os.WriteFile(tplPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	// Write BIRD peer config template
	templatesDir := "templates"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return err
	}
	peerConf := `###########################################
##                WARNING                ##
###########################################
#                                         #
#  This file is managed by iEdon PeerAPI. #
#                                         #
#  DO NOT EDIT OR DELETE THIS FILE IF YOU #
#  ARE NOT SURE.                          #
###########################################

protocol bgp {{ .SessionName }} from dnpeers {
    neighbor {{ .InterfaceAddr }} as {{ .ASN }};
    {{- if .SourceAddress }}
    source address {{ .SourceAddress }};
    {{- end }}
    {{- if .ExtendedNextHopOn }}
    ipv4 { extended next hop on; };
    {{- end }};
}
`
	if err := os.WriteFile(filepath.Join(templatesDir, "peer.conf"), []byte(peerConf), 0644); err != nil {
		return err
	}

	fmt.Printf("Default config written to %s.\n", defaultPath)
	fmt.Println("Overlay templates (server.toml.default, bird.toml.default, sysctl.toml.default) created.")
	fmt.Println("BIRD peer config template written to templates/peer.conf.")
	fmt.Printf("Copy %s.default to %s, edit to match your node, then run again.\n", path, path)
	os.Exit(0)
	return nil
}

func writeDefaultConfig(path string) error {
	defaultConfig := `# peerapi-agent configuration
# Edit this file to match your node, then restart the agent.

[ipConfig]
ipv4 = ""
ipv6 = ""
ipv6LinkLocal = ""

[peerApiCenter]
apiUrl = "http://127.0.0.1:13000"
secret = ""
routerUuid = ""
agentSecret = ""
requestTimeout = 15
probeServerIPv4 = ""
probeServerIPv6 = ""
probeServerIPv6Prefix = ""
probeServerPort = 2189
heartbeatInterval = 30
syncInterval = 300
metricInterval = 60
wanInterfaces = ["eth0"]
sessionPassthroughJwtSecert = ""
interfaceIpAllowPublic = false
interfaceIpBlacklist = ["192.168.0.0/16","10.0.0.0/8","172.16.0.0/16","172.17.0.0/16","172.18.0.0/16","172.19.0.0/16","172.24.0.0/16","172.25.0.0/16","172.26.0.0/16","172.27.0.0/16","172.28.0.0/16","172.29.0.0/16","172.30.0.0/16","172.31.0.0/16","127.0.0.0/8","224.0.0.0/4","::1/128","ff00::/8"]

[wireguard]
wgCommandPath = "/usr/bin/wg"
privateKeyPath = "/etc/wireguard/privatekey"
publicKeyPath = "/etc/wireguard/publickey"
persistentKeepaliveInterval = 25
allowedIps = "172.16.0.0/12,10.0.0.0/8,fd00::/8,fe80::/10"
dnsUpdateInterval = 60
localEndpointHost = ""
dn42BandwidthCommunity = 24
dn42InterfaceSecurityCommunity = 34

[gre]
localEndpointHost4 = ""
localEndpointHost6 = ""
localEndpointDesc4 = ""
localEndpointDesc6 = ""
dn42BandwidthCommunity = 24
dn42InterfaceSecurityCommunity = 31

[peerProbe]
enabled = true
intervalSeconds = 300
probePacketCount = 5
probePacketIntervalMs = 100
probePacketEncryptionKey = ""
sessionWorkerCount = 32
probePacketBanner = ""
probeSummaryCooldownSeconds = 30

[logger]
file = "./logs/peerapi-agent.log"
maxSize = 10
maxBackups = 10
maxAge = 30
compress = true
consoleLogging = true

[metric]
autoTeardown = true
maxMindGeoLiteCountryMmdbPath = ""
geoIpCountryMode = "blacklist"
blacklistGeoCountries = []
whitelistGeoCountries = []
pingCommandPath = "/usr/bin/ping"
pingTimeout = 5
pingCount = 2
pingCountOnFail = 1
pingWorkerCount = 64
sessionWorkerCount = 64
maxRTTMetricsHistroy = 288
geoCheckInterval = 900
filterParamsUpdateInterval = 3600
`

	return os.WriteFile(path, []byte(defaultConfig), 0644)
}
