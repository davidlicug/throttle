package quota

import (
	"k8s.io/apimachinery/pkg/api/resource"
	throttlev1alpha1 "github.com/xychu/throttle/pkg/apis/throttlecontroller/v1alpha1"
	"k8s.io/api/core/v1"
	"testing"
	"github.com/xychu/throttle/cmd/app/options"
	"github.com/xychu/throttle/pkg/util"
	"fmt"
)

func TestBuildCpuScaleUp(t *testing.T) {
	var opt = options.ServerOption{
		MaxQuotaCpu: 100,
		MinQuotaCpu: 30,
		MaxCpuRate: 0.8,
		MinCpuRate: 0.3,
		MailAdmins: "liweimin1@jd.com,niuwenjie@jd.com",
	}
	var rq  = v1.ResourceQuota {
		Status: v1.ResourceQuotaStatus{
			Hard:v1.ResourceList{
				v1.ResourceLimitsCPU: *resource.NewQuantity(5*1024*1024*1024, resource.DecimalSI),
			},
			Used:v1.ResourceList{
				v1.ResourceLimitsCPU: *resource.NewQuantity(5*1024*1024*1024, resource.DecimalSI),
			},
		},
	}

	buildCpuQuota(&opt, &rq)
}

func TestBuildCpuScaleDown(t *testing.T) {
	var opt = options.ServerOption{
		MaxQuotaCpu: 100,
		MinQuotaCpu: 30,
		MaxCpuRate: 0.8,
		MinCpuRate: 0.3,
		MailAdmins: "liweimin1@jd.com,niuwenjie@jd.com",
	}
	var rq  = v1.ResourceQuota {
		Status: v1.ResourceQuotaStatus{
			Hard:v1.ResourceList{
				v1.ResourceLimitsCPU: *resource.NewQuantity(5*1024*1024*1024, resource.DecimalSI),
			},
			Used:v1.ResourceList{
				v1.ResourceLimitsCPU: *resource.NewQuantity(5*1024*1024*1024, resource.DecimalSI),
			},
		},
	}

	buildCpuQuota(&opt, &rq)
}

func TestBuildMemoryScaleUp(t *testing.T){
	var opt = options.ServerOption{
		MaxQuotaMemory: 1000,
		MinQuotaMemory: 80,
		MaxMemoryRate: 0.8,
		MinMemoryRate: 0.3,
		MailAdmins: "liweimin1@jd.com,niuwenjie@jd.com",
	}
	var rq  = v1.ResourceQuota {
		Status: v1.ResourceQuotaStatus{
			Hard:v1.ResourceList{
				v1.ResourceLimitsMemory: *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI),
			},
			Used:v1.ResourceList{
				v1.ResourceLimitsMemory: *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI),
			},
		},
	}
	buildMemoryQuota(&opt, &rq)
}


func TestBuildMemoryScaleDown(t *testing.T){
	var opt = options.ServerOption{
		MaxQuotaMemory: 1000,
		MinQuotaMemory: 80,
		MaxMemoryRate: 0.8,
		MinMemoryRate: 0.3,
		MailAdmins: "liweimin1@jd.com,niuwenjie@jd.com",
	}
	var rq  = v1.ResourceQuota {
		Status: v1.ResourceQuotaStatus{
			Hard:v1.ResourceList{
				v1.ResourceLimitsMemory: *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI),
			},
			Used:v1.ResourceList{
				v1.ResourceLimitsMemory: *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI),
			},
		},
	}
	buildMemoryQuota(&opt, &rq)
}

func TestBuildGpuScaleUp(t *testing.T) {
	var opt = options.ServerOption{
		MaxQuotaGpu: 100,
		MinQuotaGpu: 30,
		MaxGpuRate: 0.8,
		MinGpuRate: 0.3,
		MailAdmins: "liweimin1@jd.com,niuwenjie@jd.com",
	}
	var rq  = throttlev1alpha1.GPUQuota {
		Status: v1.ResourceQuotaStatus{
			Hard: v1.ResourceList{
				throttlev1alpha1.ResourceRequestsGPU: *resource.NewQuantity(5*1024*1024*1024, resource.DecimalSI),
			},
			Used: v1.ResourceList{
				throttlev1alpha1.ResourceLimitsGPU: *resource.NewQuantity(5*1024*1024*1024, resource.DecimalSI),
			},
		},
	}

	buildGpuQuota(&opt, &rq)
}

func TestBuildGpuScaleDown(t *testing.T) {
	var opt = options.ServerOption{
		MaxQuotaGpu: 100,
		MinQuotaGpu: 30,
		MaxGpuRate: 0.8,
		MinGpuRate: 0.3,
		MailAdmins: "liweimin1@jd.com,niuwenjie@jd.com",
	}
	var rq  = throttlev1alpha1.GPUQuota {
		Status: v1.ResourceQuotaStatus{
			Hard: v1.ResourceList{
				throttlev1alpha1.ResourceRequestsGPU: *resource.NewQuantity(5*1024*1024*1024, resource.DecimalSI),
			},
			Used: v1.ResourceList{
				throttlev1alpha1.ResourceLimitsGPU: *resource.NewQuantity(5*1024*1024*1024, resource.DecimalSI),
			},
		},
	}

	buildGpuQuota(&opt, &rq)
}


func TestSendMail(t *testing.T) {
	name := "ads-model-pinocloud"
	users, err := util.MailUsers(requestUserEmailServerAddr + name)
	if err != nil {
		t.Errorf("get mail users from resource quota:%s", name)
		return
	}
	subject := fmt.Sprintf("9NCloud任务资源配额自适应调整，CPU资源利用率高于%d%%", 20)
	body := fmt.Sprintf(`资源组: %s<br>
						 一周CPU平均使用率: %.2f%%<br>
						 调整前GPU配额: %dC<br>
						 调整后GPU配额: %dC<br>
						 GPU配额调整原因: 最近一周GPU平均使用率高于%d%%<br>
						 监控链接: <a href = "http://10.176.10.54/api/v1/namespaces/monitor/services/prometheus-grafana/proxy/d/HOFP871Zk/group-by-instanceid?orgId=1&from=now-1d&to=now&refresh=30s&var-InstanceId=All">点击查看</a><br>
						 <h3>资源配额调整原则</h3>
						 1、最近一周GPU平均使用率高于%d%%(不占用gpu资源请忽略)<br>
						 2、最近一周CPU平均使用率高于%d%%<br>
						 3、最近一周内存平均使用率高于%d%%<br>
						 备注：这里的使用率是指启动任务后任务的真正利用率，不真正使用资源不会统计。通过查看以往的任务监控数据合理调整任务的资源申请值，可以有效提高利用率指标<br>`,
						name, 20.0, 100, 100, 20, 20, 20, 20)
	if err := util.SendMail(users, subject, body); err != nil{
		t.Errorf("send alert mail error:%s\n", err)
	}
}


