package options

import (
	"flag"
)

// ServerOption is the main context object for the controller manager.
type ServerOption struct {
	PrintVersion		bool
	EnableCpu			bool
	EnableMemory		bool
	EnableGpu			bool
	EnableWebHook 		bool
	MasterURL			string
	Kubeconfig			string
	CertFile   			string
	KeyFile    			string
	SyncPeriod			int64
	ClusterName			string
	MaxQuotaCpu         int64
	MinQuotaCpu         int64
	MaxCpuRate			float64
	MinCpuRate			float64
	MaxQuotaMemory      int64
	MinQuotaMemory      int64
	MaxMemoryRate		float64
	MinMemoryRate		float64
	MaxQuotaGpu         int64
	MinQuotaGpu         int64
	MaxGpuRate			float64
	MinGpuRate			float64
	MailAdmins			string
	Threshold			float64
	Server				string
	Port				int
	ProxyServer 		string
	K8sLowerVersion		bool

	//ConfigFile		string
}

// NewServerOption creates a new CMServer with a default config.
func NewServerOption() *ServerOption {
	s := ServerOption{}
	return &s
}

// AddFlags adds flags for a specific CMServer to the specified FlagSet.
func (s *ServerOption) AddFlags(fs *flag.FlagSet) {
	fs.BoolVar(&s.PrintVersion, "version", false, "Show version and quit")
	fs.BoolVar(&s.EnableCpu, "enableCpu", true, "enable cpu quota resource auto scale.")
	fs.BoolVar(&s.EnableMemory, "enableMemory", true, "enable memory quota resource auto scale.")
	fs.BoolVar(&s.EnableGpu, "enableGpu", true, "enable gpu quota resource auto scale.")
	fs.BoolVar(&s.EnableWebHook, "enableWebhook", false, "enable quota resource web hook.")
	fs.StringVar(&s.MasterURL, "master", "", "ai quota master ip address.")
	fs.StringVar(&s.Kubeconfig, "kubeconfig", "", "ai quota kubeconfig.")
	fs.StringVar(&s.CertFile, "tls-cert-file", s.CertFile, ""+
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
		"after server cert).")
	fs.StringVar(&s.KeyFile, "tls-private-key-file", s.KeyFile, ""+
		"File containing the default x509 private key matching --tls-cert-file.")
	fs.Int64Var(&s.SyncPeriod, "sync-period", 10080, "ai quota sync period time per week.")
	fs.Int64Var(&s.MaxQuotaCpu, "max-cpu", 1000, "ai quota max cpu quota.")
	fs.Int64Var(&s.MinQuotaCpu, "min-cpu", 60, "ai quota min cpu quota.")
	fs.Float64Var(&s.MaxCpuRate, "max-cpu-rate", 80, "ai quota max cpu percent.")
	fs.Float64Var(&s.MinCpuRate, "min-cpu-rate", 30, "ai quota min cpu percent.")
	fs.Int64Var(&s.MaxQuotaMemory, "max-mem", 10000, "ai quota max memory quota.")
	fs.Int64Var(&s.MinQuotaMemory, "min-mem", 100, "ai quota min memory quota.")
	fs.Float64Var(&s.MaxMemoryRate, "max-mem-rate", 80, "ai quota max memory percent.")
	fs.Float64Var(&s.MinMemoryRate, "min-mem-rate", 40, "ai quota min memory percent.")
	fs.Int64Var(&s.MaxQuotaGpu, "max-gpu", 100, "ai quota max gpu quota.")
	fs.Int64Var(&s.MinQuotaGpu, "min-gpu", 0, "ai quota min gpu quota.")
	fs.Float64Var(&s.MaxGpuRate, "max-gpu-rate", 80, "ai quota max gpu percent.")
	fs.Float64Var(&s.MinGpuRate, "min-gpu-rate", 10, "ai quota min gpu percent.")
	fs.StringVar(&s.MailAdmins, "mail-admins", "liweimin1@jd.com,niuwenjie@jd.com,likairong@jd.com", "ai quota receive mail users.")
	fs.Float64Var(&s.Threshold, "threshold", 0.6, "ai quota admin pod threshold.")
	fs.StringVar(&s.ClusterName, "cluster", "jdcloud-dev-zyx", "ai quota cluster name.")				//jdcloud-dev-zyx lang-fang
	fs.StringVar(&s.Server, "server", "192.168.30.206", "ai quota prometheus server address.")			//192.168.30.206 10.254.162.125
	fs.IntVar(&s.Port, "port", 9090, "ai quota prometheus server port.")
	fs.StringVar(&s.ProxyServer, "proxy-server", "10.176.1.20", "ai quota prometheus proxy server.")	//10.176.1.20 172.18.178.107
	fs.BoolVar(&s.K8sLowerVersion, "k8s-low-version", false, "cluster k8s version")						//false, true

	//fs.StringVar(&s.ConfigFile, "config", "/etc/ai-ops/ai-ops.conf", "ai ops config file.")
}