package app

import (
	"os"

	"github.com/sirupsen/logrus"

	"github.com/xychu/throttle/cmd/app/options"
	"github.com/xychu/throttle/pkg/version"
	"github.com/xychu/throttle/pkg/quota"
	"github.com/xychu/throttle/pkg/util"
)

const (
	apiVersion = "v0.1.0"
	RecommendedKubeConfigPathEnv = "KUBECONFIG"
	reSyncResourceQuotaDefaultPeriod = 24 * 7 * 60
)


func Run(opt *options.ServerOption) error {
	if opt.PrintVersion {
		version.PrintVersionAndExit(apiVersion)
	}

	// Note: ENV KUBECONFIG will overwrite user defined Kubeconfig option.
	if len(os.Getenv(RecommendedKubeConfigPathEnv)) > 0 {
		// use the current context in kubeconfig
		// This is very useful for running locally.
		opt.Kubeconfig = os.Getenv(RecommendedKubeConfigPathEnv)
	}

	if opt.SyncPeriod <=0 {
		opt.SyncPeriod = int64(reSyncResourceQuotaDefaultPeriod)
	}

	logrus.Infof("server option:%#v\n", *opt)

	c, err := util.NewK8sClient(opt)
	if err != nil {
		logrus.Errorf("create k8s client error:%s\n", err)
		return err
	}

	quota.InitK8sClient(c)
	quota.InitNsWhiteList(opt.WhiteList)

	if opt.EnableWebHook {
		go quota.StartWebHook(opt)
	}

	if err := quota.SyncBuildQuotaResource(opt); err != nil {
		logrus.Errorf("sync build quota resource error:%s\n", err)
		return err
	}


	return nil
}

