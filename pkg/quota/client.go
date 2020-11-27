package quota

import (
	"time"
	"strings"
	"net/http"
	"crypto/tls"

	"github.com/sirupsen/logrus"
	"github.com/xychu/throttle/cmd/app/options"
	"github.com/xychu/throttle/pkg/util"
)

var globalK8sClient *util.K8sClient

var nsWhiteList []string

func InitK8sClient(client *util.K8sClient){
	globalK8sClient = client
}

func InitNsWhiteList(str string){
	nsWhiteList = strings.Split(str, ",")
}

func SyncBuildQuotaResource(opt *options.ServerOption)error{
	for range time.Tick(time.Duration(opt.SyncPeriod) * time.Minute) {
		logrus.Infof("start to sync build quota resource period: %d\n", opt.SyncPeriod)
		if err := BuildQuotaResource(opt); err != nil {
			logrus.Errorf("build quota resource error:%s\n", err)
		}
	}

	return nil
}

func BuildQuotaResource(opt *options.ServerOption)error{
	if opt.K8sLowerVersion {
		if err := buildResourceQuotaInLowK8s(opt); err != nil {
			logrus.Errorf("build resource quota in k8s lower version error:%s", err)
			return err
		}
	}else {
		if err := buildResourceQuotaInHighK8s(opt); err != nil {
			logrus.Errorf("build resource quota in k8s version error:%s", err)
			return err
		}
	}


	return nil
}

func StartWebHook(opt *options.ServerOption){
	InitThreshold(opt.Threshold)
	http.HandleFunc("/", ServePods)
	server := &http.Server{
		Addr:      ":443",
		TLSConfig: configTLS(opt),
	}
	server.ListenAndServeTLS("", "")
	return
}

func configTLS(opt *options.ServerOption) *tls.Config {
	sCert, err := tls.LoadX509KeyPair(opt.CertFile, opt.KeyFile)
	if err != nil {
		logrus.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
		// TODO: uses mutual tls after we agree on what cert the apiserver should use.
		// ClientAuth:   tls.RequireAndVerifyClientCert,
	}
}