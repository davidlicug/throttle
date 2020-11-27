package quota

import (
	"fmt"
	"time"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/xychu/throttle/cmd/app/options"
	"github.com/xychu/throttle/pkg/util"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	throttlev1alpha1 "github.com/xychu/throttle/pkg/apis/throttlecontroller/v1alpha1"
)

const (
	scaleDownMultiple = 0.8
	scaleUpMultiple = 1.2

	samplingIntervalMinute = 2
	samplingIntervalSecond = samplingIntervalMinute * 60

	memoryOneGiByte = 1024 * 1024 * 1024

	prefixParameters = "/api/v1/query_range?query="
	requestUserEmailServerAddr = "http://172.28.217.170/groups/"

	resourceRequestGpu = "requests.nvidia.com/gpu"

	relativeError = 4.0

	resourceQuotaWhiteList = "whitelist"
)

var (
	cpuAvgUsageOneWeekQueryParameters = `(sum (rate (container_cpu_usage_seconds_total{image!="",name=~"^k8s_.*",pod_name!="",namespace="%s"}[1m])) by (namespace))/(sum(kube_pod_container_resource_requests_cpu_cores{pod!="",node!="",namespace="%s",pod_phase=~"Pending|Running"})by(namespace))*100&start=%d&end=%d&step=%d`
	memAvgUsageOneWeekQueryParameters = `(sum (container_memory_usage_bytes{image!="",name=~"^k8s_.*",pod_name!="",namespace="%s"})by(namespace)) /(sum(kube_pod_container_resource_requests_memory_bytes{pod!="",node!="",namespace="%s",pod_phase=~"Pending|Running"})by(namespace))*100&start=%d&end=%d&step=%d`
	gpuAvgUsageOneWeekQueryParameters = `(sum(container_accelerator_duty_cycle{image!="",name=~"^k8s_.*",pod_name!="",pod_name!~"nvidia-device-plugin-daemonset.*",namespace="%s"}/100)by(namespace))/(count(container_accelerator_duty_cycle{image!="",name=~"^k8s_.*",pod_name!="",pod_name!~"nvidia-device-plugin-daemonset.*",namespace="%s"})by(namespace))*100&start=%d&end=%d&step=%d`
	monitorServerAddress = `http://%s/api/v1/namespaces/monitor/services/prometheus-grafana/proxy/dashboard/db/namespace-resources-usage?orgId=1&var-Namespace=%s&var-Resolution=%dm`

	alertCpuEmail = `集群: %s<br>
					资源组: %s<br>
					一周CPU平均使用率: %.2f%%<br>
					调整前CPU配额: %dC<br>
					调整后CPU配额: %dC<br>
					CPU配额调整原因: 最近一周CPU平均使用率低于%.f%%<br>
					监控链接: <a href = "%s">点击查看</a><br>
						<h3>资源配额调整原则</h3>
					1. 最近一周GPU平均使用率低于%.f%%(不占用gpu资源请忽略)<br>
					2. 最近一周CPU平均使用率低于%.f%%<br>
					3. 最近一周内存平均使用率低于%.f%%<br>
					<h4>备注：<br>
					1. 这里的使用率是指启动任务后任务的真正利用率，不真正使用资源不会统计。通过查看以往的任务监控数据合理调整任务的资源申请值，并且减小任务完成后的保留时间，可以有效提高使用率指标。<br>
					2. 一个group可以有多个集群（cluster）权限，每个cluster会单独调整，某group在某个集群的quota被调降，说明该集群的任务资源占用需要优化，不影响该group在其他集群的配额。<br>
					3. 使用率经过优化达标后，如何申请上调?<br>
					邮件说明资源组等相关情况，请直属上级审批，通过后调整为当前值的%.f%%，但不会超过未下调前的最初原始资源量</h4>`
	alertMemoryEmail = `集群: %s<br>
						资源组: %s<br>
						内存使用率: %.2f%%<br>
						调整前内存配额: %.2fG<br>
						调整后内存配额: %.2fG<br>
						内存配额调整原因: 最近一周内存平均使用率低于%.f%%<br>
						监控链接: <a href = "%s">点击查看</a><br>
						<h3>资源配额自适应调整策略</h3>
						1. 最近一周GPU平均使用率低于%.f%%(不占用gpu资源请忽略)<br>
						2. 最近一周CPU平均使用率低于%.f%%<br>
						3. 最近一周内存平均使用率低于%.f%%<br>
						<h4>备注：<br>
						1. 这里的使用率是指启动任务后任务的真正利用率，不真正使用资源不会统计。通过查看以往的任务监控数据合理调整任务的资源申请值，并且减小任务完成后的保留时间，可以有效提高使用率指标。<br>
						2. 一个group可以有多个集群（cluster）权限，每个cluster会单独调整，某group在某个集群的quota被调降，说明该集群的任务资源占用需要优化，不影响该group在其他集群的配额。<br>
						3. 使用率经过优化达标后，如何申请上调?<br>
						邮件说明资源组等相关情况，请直属上级审批，通过后调整为当前值的%.f%%，但不会超过未下调前的最初原始资源量</h4>`
	alertGpuEmail = `集群: %s<br>
					资源组: %s<br>
					一周GPU平均使用率: %.2f%%<br>
					调整前GPU配额: %d<br>
					调整后GPU配额: %d<br>
					GPU配额调整原因: 最近一周GPU平均使用率低于%.f%%<br>
					监控链接: <a href = "%s">点击查看</a><br>
					<h3>资源配额调整原则</h3>
					1. 最近一周GPU平均使用率低于%.f%%(不占用gpu资源请忽略)<br>
					2. 最近一周CPU平均使用率低于%.f%%<br>
					3. 最近一周内存平均使用率低于%.f%%<br>
					<h4>备注：<br>
					1. 这里的使用率是指启动任务后任务的真正利用率，不真正使用资源不会统计。通过查看以往的任务监控数据合理调整任务的资源申请值，并且减小任务完成后的保留时间，可以有效提高使用率指标。<br>
					2. 一个group可以有多个集群（cluster）权限，每个cluster会单独调整，某group在某个集群的quota被调降，说明该集群的任务资源占用需要优化，不影响该group在其他集群的配额。<br>
					3. 使用率经过优化达标后，如何申请上调?<br>
					邮件说明资源组等相关情况，请直属上级审批，通过后调整为当前值的%.f%%，但不会超过未下调前的最初原始资源量</h4>`
	alertCpuEmailTitle = `9NCloud资源组配额下调，CPU资源使用率低于%.f%%`
	alertMemoryEmailTitle = `9NCloud资源组配额下调，内存资源使用率低于%.f%%`
	alertGpuEmailTitle = `9NCloud资源组配额下调，GPU资源使用率低于%.f%%`
)

func buildResourceQuotaInLowK8s(opt *options.ServerOption)error{
	nsList, err := globalK8sClient.Client.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("get all namespaaces error:%s", err)
	}
	for _, ns := range nsList.Items {
		go func(namespace string){
			users, err := util.MailUsers(requestUserEmailServerAddr + namespace)
			if err != nil {
				logrus.Errorf("get mail users by namespace:%s error:%s", namespace, err)
				return
			}
			if len(users) < 1 {
				logrus.Infof("no mail users in namespace:%s", namespace)
				return
			}
			rqList, err := globalK8sClient.Client.CoreV1().ResourceQuotas(namespace).List(metav1.ListOptions{})
			if err != nil {
				logrus.Errorf("get resource quota in namespace:%s", namespace)
				return
			}
			hrefUrl := fmt.Sprintf(monitorServerAddress, opt.ProxyServer, namespace, samplingIntervalMinute)
			for _, rq := range rqList.Items {
				if rq.Labels != nil {
					if _, ok := rq.Labels[resourceQuotaWhiteList]; ok {
						continue
					}
				}
				var (
					bcpu, bmem bool
					preCpu, curCpu, preMem, curMem int64
					cpuUsage, memUsage float64
				)
				if opt.EnableCpu {
					preCpu, curCpu, cpuUsage, bcpu = buildCpuQuota(opt, &rq)
				}
				if opt.EnableMemory {
					preMem, curMem, memUsage, bmem = buildMemoryQuota(opt, &rq)
				}

				admins := strings.Split(opt.MailAdmins, ",")
				admins = append(admins, users...)
				admins = util.RemoveRepByMap(admins)
				if bcpu || bmem {
					err := updateResourceQuota(rq)
					if err != nil {
						logrus.Errorf("update resource quota:%s/%s error:%s\n", namespace, rq.Name, err)
						continue
					}

					if bcpu {
						subject := fmt.Sprintf(alertCpuEmailTitle, opt.MinCpuRate)
						body := fmt.Sprintf(alertCpuEmail,
							opt.ClusterName,
							rq.Name,
							cpuUsage,
							preCpu,
							curCpu,
							opt.MinCpuRate,
							hrefUrl,
							opt.MinGpuRate,
							opt.MinCpuRate,
							opt.MinMemoryRate,
							scaleUpMultiple * 100)
						util.SendMail(admins, subject, body)
					}
					if bmem {
						subject := fmt.Sprintf(alertMemoryEmailTitle, opt.MinMemoryRate)
						body := fmt.Sprintf(alertMemoryEmail,
							opt.ClusterName,
							rq.Name,
							memUsage,
							float64(preMem)/memoryOneGiByte,
							float64(curMem)/memoryOneGiByte,
							opt.MinMemoryRate,
							hrefUrl,
							opt.MinGpuRate,
							opt.MinCpuRate,
							opt.MinMemoryRate,
							scaleUpMultiple * 100)
						util.SendMail(admins, subject, body)
					}
				}
			}
		}(ns.Name)
		go func(namespace string){
			if opt.EnableGpu {
				users, err := util.MailUsers(requestUserEmailServerAddr + namespace)
				if err != nil {
					logrus.Errorf("get mail users by namespace:%s error:%s", namespace, err)
					return
				}
				if len(users) < 1 {
					logrus.Infof("no mail users in namespace:%s", namespace)
					return
				}
				rqList, err := globalK8sClient.ThrottleClient.ThrottlecontrollerV1alpha1().GPUQuotas(namespace).List(metav1.ListOptions{})
				if err != nil {
					logrus.Errorf("get gpu resource quota in namespace:%s error:%s", namespace, err)
					return
				}
				hrefUrl := fmt.Sprintf(monitorServerAddress, opt.ProxyServer, namespace, samplingIntervalMinute)
				for _, rq := range rqList.Items {
					if rq.Labels != nil {
						if _, ok := rq.Labels[resourceQuotaWhiteList]; ok {
							continue
						}
					}
					var (
						bgpu bool
						preGpu, curGpu int64
						gpuUsage float64
					)
					preGpu, curGpu, gpuUsage, bgpu = buildGpuQuota(opt, &rq)
					admins := strings.Split(opt.MailAdmins, ",")
					admins = append(admins, users...)
					admins = util.RemoveRepByMap(admins)
					if bgpu {
						if _, err := globalK8sClient.ThrottleClient.ThrottlecontrollerV1alpha1().GPUQuotas(namespace).Update(&rq); err != nil {
							logrus.Errorf("update gpu quota:%s/%s error:%s", namespace, rq.Name, err)
							continue
						}
						subject := fmt.Sprintf(alertGpuEmailTitle, opt.MinGpuRate)
						body := fmt.Sprintf(alertGpuEmail,
							opt.ClusterName,
							rq.Name,
							gpuUsage,
							preGpu,
							curGpu,
							opt.MinGpuRate,
							hrefUrl,
							opt.MinGpuRate,
							opt.MinCpuRate,
							opt.MinMemoryRate,
							scaleUpMultiple * 100)
						util.SendMail(admins, subject, body)
					}
				}
			}
		}(ns.Name)
	}

	return nil
}

func buildResourceQuotaInHighK8s(opt *options.ServerOption)error{
	nsList, err := globalK8sClient.Client.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("get all namespaaces error:%s", err)
	}
	for _, ns := range nsList.Items {
		go func(namespace string){
			users, err := util.MailUsers(requestUserEmailServerAddr + namespace)
			if err != nil {
				logrus.Errorf("get mail users by namespace:%s error:%s", namespace, err)
				return
			}
			if len(users) < 1 {
				logrus.Infof("no mail users in namespace:%s", namespace)
				return
			}
			rqList, err := globalK8sClient.Client.CoreV1().ResourceQuotas(namespace).List(metav1.ListOptions{})
			if err != nil {
				logrus.Errorf("get resource quota in namespace:%s", namespace)
				return
			}
			hrefUrl := fmt.Sprintf(monitorServerAddress, opt.ProxyServer, namespace, samplingIntervalMinute)
			for _, rq := range rqList.Items {
				if rq.Labels != nil {
					if _, ok := rq.Labels[resourceQuotaWhiteList]; ok {
						continue
					}
				}
				var (
					bcpu, bmem, bgpu bool
					preCpu, curCpu, preMem, curMem, preGpu, curGpu int64
					cpuUsage, memUsage, gpuUsage float64
				)
				if opt.EnableCpu {
					preCpu, curCpu, cpuUsage, bcpu = buildCpuQuota(opt, &rq)
					logrus.Infof("namespace:%s cpu average usage:%f pre cpu:%d cur cpu:%d changed:%t", namespace, cpuUsage, preCpu, curCpu, bcpu)
				}
				if opt.EnableMemory {
					preMem, curMem, memUsage, bmem = buildMemoryQuota(opt, &rq)
					logrus.Infof("namespace:%s memory average usage:%f pre memory:%d cur memory:%d changed:%t", namespace, memUsage, preMem, curMem, bmem)
				}
				if opt.EnableGpu {
					preGpu, curGpu, gpuUsage, bgpu = buildGpuQuota(opt, &rq)
					logrus.Infof("namespace:%s gpu average usage:%f pre gpu:%d cur gpu:%d changed:%t", namespace, gpuUsage, preGpu, curGpu, bgpu)
				}

				admins := strings.Split(opt.MailAdmins, ",")
				admins = append(admins, users...)
				admins = util.RemoveRepByMap(admins)
				if bcpu || bmem || bgpu {
					err := updateResourceQuota(rq)
					if err != nil {
						logrus.Errorf("update resource quota:%s/%s error:%s\n", namespace, rq.Name, err)
						continue
					}

					if bcpu {
						subject := fmt.Sprintf(alertCpuEmailTitle, opt.MinCpuRate)
						body := fmt.Sprintf(alertCpuEmail,
							opt.ClusterName,
							rq.Name,
							cpuUsage,
							preCpu,
							curCpu,
							opt.MinCpuRate,
							hrefUrl,
							opt.MinGpuRate,
							opt.MinCpuRate,
							opt.MinMemoryRate,
							scaleUpMultiple * 100)
						util.SendMail(admins, subject, body)
					}
					if bmem {
						subject := fmt.Sprintf(alertMemoryEmailTitle, opt.MinMemoryRate)
						body := fmt.Sprintf(alertMemoryEmail,
							opt.ClusterName,
							rq.Name,
							memUsage,
							float64(preMem)/memoryOneGiByte,
							float64(curMem)/memoryOneGiByte,
							opt.MinMemoryRate,
							hrefUrl,
							opt.MinGpuRate,
							opt.MinCpuRate,
							opt.MinMemoryRate,
							scaleUpMultiple * 100)
						util.SendMail(admins, subject, body)
					}
					if bgpu {
						subject := fmt.Sprintf(alertGpuEmailTitle, opt.MinGpuRate)
						body := fmt.Sprintf(alertGpuEmail,
							opt.ClusterName,
							rq.Name,
							gpuUsage,
							preGpu,
							curGpu,
							opt.MinGpuRate,
							hrefUrl,
							opt.MinGpuRate,
							opt.MinCpuRate,
							opt.MinMemoryRate,
							scaleUpMultiple * 100)
						util.SendMail(admins, subject, body)
					}
				}
			}
		}(ns.Name)
	}

	return nil
}

func buildCpuQuota(opt *options.ServerOption, rq *v1.ResourceQuota)(int64, int64, float64, bool){
	var (
		pre, cur int64
		usage float64
		isChanged bool
	)
	if rq.Status.Hard == nil || rq.Status.Used == nil{
		logrus.Errorf("buildCpuQuota:resource quota:%s status is null\n", rq.Name)
		return pre, cur, usage, isChanged
	}
	now := time.Now()
	begin := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -7)
	urlStr := fmt.Sprintf("http://%s:%d", opt.Server, opt.Port)
	urlStr += prefixParameters + fmt.Sprintf(cpuAvgUsageOneWeekQueryParameters, rq.Namespace, rq.Namespace, begin.Local().Unix(), now.Local().Unix(), samplingIntervalSecond)
	usage, err := calcNsAvgUsage(urlStr)
	if err != nil {
		logrus.Warnf("buildCpuQuota:namespace:%s calc ns cpu average usage by url:%s error:%s\n", rq.Namespace, urlStr, err)
		return pre, cur, usage, isChanged
	}
	totalCpu, ok := rq.Status.Hard[v1.ResourceLimitsCPU]
	if !ok{
		logrus.Errorf("buildCpuQuota:resource quota:%s status.hard.%s is null\n", rq.Name, v1.ResourceLimitsCPU)
		return pre, cur, usage, isChanged
	}

	pre, _ = totalCpu.AsInt64()
	if usage < opt.MinCpuRate && pre > opt.MinQuotaCpu {
		new := float64(pre) * scaleDownMultiple
		if int64(new) < opt.MinQuotaCpu {
			new = float64(opt.MinQuotaCpu)
		}
		cur = int64(new)
		totalCpu.Set(cur)
		rq.Spec.Hard[v1.ResourceLimitsCPU] = totalCpu
		isChanged = true
	}

	return pre, cur, usage, isChanged
}

func buildMemoryQuota(opt *options.ServerOption, rq *v1.ResourceQuota)(int64, int64, float64, bool){
	var (
		pre, cur int64
		usage float64
		isChanged bool
	)
	if rq.Status.Hard == nil || rq.Status.Used == nil{
		logrus.Errorf("buildMemoryQuota:resource quota:%s status is null\n", rq.Name)
		return pre, cur, usage, isChanged
	}
	now := time.Now()
	begin := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -7)
	urlStr := fmt.Sprintf("http://%s:%d", opt.Server, opt.Port)
	urlStr += prefixParameters + fmt.Sprintf(memAvgUsageOneWeekQueryParameters, rq.Namespace, rq.Namespace, begin.Local().Unix(), now.Local().Unix(), samplingIntervalSecond)
	usage, err := calcNsAvgUsage(urlStr)
	if err != nil {
		logrus.Warnf("buildMemoryQuota:namespace:%s calc ns memory average usage by url:%s error:%s\n", rq.Namespace, urlStr, err)
		return pre, cur, usage, isChanged
	}
	totalMemory, ok := rq.Status.Hard[v1.ResourceLimitsMemory]
	if !ok{
		logrus.Errorf("buildMemoryQuota:resource quota:%s status.hard.%s is null\n", rq.Name, v1.ResourceLimitsCPU)
		return pre, cur, usage, isChanged
	}

	pre, _ = totalMemory.AsInt64()
	if usage + relativeError < opt.MinMemoryRate && int64(float64(pre)/memoryOneGiByte) > opt.MinQuotaMemory {
		new := float64(pre) * scaleDownMultiple
		if int64(new) < opt.MinQuotaMemory {
			new = float64(opt.MinQuotaMemory)
		}
		cur = int64(new)
		totalMemory.Set(cur)
		rq.Spec.Hard[v1.ResourceLimitsMemory] = totalMemory
		isChanged = true
	}

	return pre, cur, usage, isChanged
}

func buildGpuQuota(opt *options.ServerOption, rs interface{})(int64, int64, float64, bool){
	if tgq, ok := rs.(*throttlev1alpha1.GPUQuota); ok {
		if tgq != nil {
			var (
				pre, cur int64
				usage float64
				isChanged bool
			)
			if tgq.Status.Hard == nil || tgq.Status.Used == nil{
				logrus.Errorf("buildGpuQuota:resource quota:%s status is null\n", tgq.Name)
				return pre, cur, usage, isChanged
			}
			now := time.Now()
			begin := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -7)
			urlStr := fmt.Sprintf("http://%s:%d", opt.Server, opt.Port)
			urlStr += prefixParameters + fmt.Sprintf(gpuAvgUsageOneWeekQueryParameters, tgq.Namespace, tgq.Namespace, begin.Local().Unix(), now.Local().Unix(), samplingIntervalSecond)
			usage, err := calcNsAvgUsage(urlStr)
			if err != nil {
				logrus.Warnf("buildGpuQuota:namespace:%s calc ns gpu average usage by url:%s error:%s\n", tgq.Namespace, urlStr, err)
				return pre, cur, usage, isChanged
			}
			totalGpu, ok := tgq.Status.Hard[throttlev1alpha1.ResourceLimitsGPU]
			if !ok{
				logrus.Errorf("buildGpuQuota:resource quota:%s status.hard.%s is null\n", tgq.Name, throttlev1alpha1.ResourceLimitsGPU)
				return pre, cur, usage, isChanged
			}

			pre, _ = totalGpu.AsInt64()
			if usage < opt.MinGpuRate && pre > opt.MinQuotaGpu {
				new := float64(pre) * scaleDownMultiple
				if int64(new) < opt.MinQuotaGpu {
					new = float64(opt.MinQuotaGpu)
				}
				cur = int64(new)
				totalGpu.Set(cur)
				tgq.Spec.Hard[throttlev1alpha1.ResourceLimitsGPU] = totalGpu
				tgq.Spec.Hard[throttlev1alpha1.ResourceRequestsGPU] = totalGpu
				isChanged = true
			}

			return pre, cur, usage, isChanged
		}
	}else if grq, ok := rs.(*v1.ResourceQuota); ok {
		if grq != nil {
			var (
				pre, cur int64
				usage float64
				isChanged bool
			)
			if grq.Status.Hard == nil || grq.Status.Used == nil{
				logrus.Errorf("buildGpuQuota:resource quota:%s status is null\n", grq.Name)
				return pre, cur, usage, isChanged
			}
			now := time.Now()
			begin := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -7)
			urlStr := fmt.Sprintf("http://%s:%d", opt.Server, opt.Port)
			urlStr += prefixParameters + fmt.Sprintf(gpuAvgUsageOneWeekQueryParameters, grq.Namespace, grq.Namespace, begin.Local().Unix(), now.Local().Unix(), samplingIntervalSecond)
			usage, err := calcNsAvgUsage(urlStr)
			if err != nil {
				logrus.Warnf("buildGpuQuota:namespace:%s calc ns gpu average usage by url:%s error:%s\n", grq.Namespace, urlStr, err)
				return pre, cur, usage, isChanged
			}
			totalGpu, ok := grq.Status.Hard[resourceRequestGpu]
			if !ok{
				logrus.Errorf("buildGpuQuota:resource quota:%s status.hard.%s is null\n", grq.Name, resourceRequestGpu)
				return pre, cur, usage, isChanged
			}

			pre, _ = totalGpu.AsInt64()
			if usage < opt.MinGpuRate && pre > opt.MinQuotaGpu {
				new := float64(pre) * scaleDownMultiple
				if int64(new) < opt.MinQuotaGpu {
					new = float64(opt.MinQuotaGpu)
				}
				cur = int64(new)
				totalGpu.Set(cur)
				grq.Spec.Hard[resourceRequestGpu] = totalGpu
				isChanged = true
			}
			return pre, cur, usage, isChanged
		}
	}

	return 0, 0, 0, false
}


func updateResourceQuota(rq v1.ResourceQuota)error{
	if _, err := globalK8sClient.Client.CoreV1().ResourceQuotas(rq.Namespace).Update(&rq); err != nil {
		return err
	}

	return nil
}

