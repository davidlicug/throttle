package quota

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"github.com/sirupsen/logrus"
	"math"
)

const (
	masterNodeLabelKey = "node-role.kubernetes.io/master"
	minPositiveNum = 0.0000001
)

var scheme = runtime.NewScheme()
var codecs = serializer.NewCodecFactory(scheme)
var adminPodThreshold float64

func InitThreshold(threshold float64){
	adminPodThreshold = threshold
}

func init() {
	addToScheme(scheme)
}

func addToScheme(scheme *runtime.Scheme) {
	corev1.AddToScheme(scheme)
	admissionregistrationv1beta1.AddToScheme(scheme)
}

func toAdmissionResponse(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

func calcPodCPUUsage(pod *corev1.Pod) (resource.Quantity, resource.Quantity) {
	requestValue := resource.Quantity{}
	limitValue := resource.Quantity{}
	for j := range pod.Spec.Containers {
		logrus.Infof("calc container resource: '%v'", pod.Spec.Containers[j].Resources)
		if request, found := pod.Spec.Containers[j].Resources.Requests[corev1.ResourceCPU]; found {
			requestValue.Add(request)
		}
		if limit, found := pod.Spec.Containers[j].Resources.Limits[corev1.ResourceCPU]; found {
			limitValue.Add(limit)
		}
	}
	// InitContainers are run **sequentially** before other containers start, so the highest
	// init container resource is compared against the sum of app containers to determine
	// the effective usage for both requests and limits.
	for j := range pod.Spec.InitContainers {
		if request, found := pod.Spec.InitContainers[j].Resources.Requests[corev1.ResourceCPU]; found {
			if requestValue.Cmp(request) < 0 {
				requestValue = request.DeepCopy()
			}
		}
		if limit, found := pod.Spec.InitContainers[j].Resources.Limits[corev1.ResourceCPU]; found {
			if limitValue.Cmp(limit) < 0 {
				limitValue = limit.DeepCopy()
			}
		}
	}
	return requestValue, limitValue
}

func calcClusterCPUUsage()(resource.Quantity, resource.Quantity, error){
	totalCpu := resource.Quantity{}
	usedCpu := resource.Quantity{}
	nodeList, err := globalK8sClient.Client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("get nodes error:%s\n", err)
		return totalCpu, usedCpu, err
	}

	podList, err := globalK8sClient.Client.CoreV1().Pods(corev1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("get nodes error:%s\n", err)
		return totalCpu, usedCpu, err
	}

	for _, node := range nodeList.Items {
		if _, ok := node.Labels[masterNodeLabelKey]; !ok {
			totalCpu.Add(*node.Status.Capacity.Cpu())
		}
	}

	for _, pod := range podList.Items {
		_, limitValue := calcPodCPUUsage(&pod)
		usedCpu.Add(limitValue)
	}

	return totalCpu, usedCpu, nil
}

func calcResourceQuotaCPUUsage()(resource.Quantity, resource.Quantity, error){
	totalCpu := resource.Quantity{}
	usedCpu := resource.Quantity{}
	rqList, err := globalK8sClient.Client.CoreV1().ResourceQuotas(corev1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("get resource quotas error:%s\n", err)
		return totalCpu, usedCpu, err
	}
	for _, rq := range rqList.Items {
		if cpu, found := rq.Status.Hard[corev1.ResourceLimitsCPU]; found {
			totalCpu.Add(cpu)
		}
		if cpu, found := rq.Status.Used[corev1.ResourceLimitsCPU]; found {
			usedCpu.Add(cpu)
		}
	}

	return totalCpu, usedCpu, nil
}

func admitPod(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse{
	logrus.Info("admitting pods")
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		err := fmt.Errorf("expect resource to be %s", podResource)
		logrus.Error(err)
		return toAdmissionResponse(err)
	}

	isUpdate := false
	if &ar.Request.OldObject != nil {
		isUpdate = true
	}
	raw := ar.Request.Object.Raw
	oldRaw := []byte{}
	pod := corev1.Pod{}
	oldPod := corev1.Pod{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
		logrus.Error(err)
		return toAdmissionResponse(err)
	}
	if isUpdate {
		oldRaw = ar.Request.OldObject.Raw
		if _, _, err := deserializer.Decode(oldRaw, nil, &oldPod); err != nil {
			logrus.Error(err)
			return toAdmissionResponse(err)
		}
	}

	clusterTotalCpu, clusterUsedCpu, err := calcClusterCPUUsage()
	if err != nil {
		err := fmt.Errorf("calc cluster cpu usage:%s", err)
		logrus.Error(err)
		return toAdmissionResponse(err)
	}

	quotaTotalCpu, quotaUsedCpu, err := calcResourceQuotaCPUUsage()
	if err != nil {
		err := fmt.Errorf("calc resource quota cpu usage:%s", err)
		logrus.Error(err)
		return toAdmissionResponse(err)
	}

	var overSold, busy float64
	if quotaTotalCpu.Cmp(clusterTotalCpu) > 0  {
		quota, _ := quotaTotalCpu.AsInt64()
		clusterTotal, _ := clusterTotalCpu.AsInt64()
		clusterUsed, _ := clusterUsedCpu.AsInt64()
		overSold = float64(quota) / float64(clusterTotal)
		busy = float64(clusterUsed) / float64(clusterTotal)
	}

	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true

	var msg string
	if overSold > 1 {
		total, _ := quotaTotalCpu.AsInt64()
		used, _ := quotaUsedCpu.AsInt64()
		quotaUsage := float64(used) / float64(total)
		base := int64(float64(total) / overSold)
		if used > base {
			score := math.Log(1 / (float64(used) + minPositiveNum)) / (math.Log(1 / (1 - busy + minPositiveNum)) + (math.Log(1 / ( quotaUsage + minPositiveNum))))
			if score < adminPodThreshold {
				reviewResponse.Allowed = false
				msg += fmt.Sprintf("cluster resource is busy")
			}
		}
	}

	if !reviewResponse.Allowed {
		reviewResponse.Result = &metav1.Status{Message: strings.TrimSpace(msg)}
	}

	 return &reviewResponse
}

// only allow pods to pull images from specific registry.
func admitPods(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	logrus.Info("admitting pods")
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		err := fmt.Errorf("expect resource to be %s", podResource)
		logrus.Error(err)
		return toAdmissionResponse(err)
	}

	isUpdate := false
	if &ar.Request.OldObject != nil {
		isUpdate = true
	}
	raw := ar.Request.Object.Raw
	oldRaw := []byte{}
	pod := corev1.Pod{}
	oldPod := corev1.Pod{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
		logrus.Error(err)
		return toAdmissionResponse(err)
	}
	if isUpdate {
		oldRaw = ar.Request.OldObject.Raw
		if _, _, err := deserializer.Decode(oldRaw, nil, &oldPod); err != nil {
			logrus.Error(err)
			return toAdmissionResponse(err)
		}
	}
	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true

	requestCPUQuota := resource.Quantity{}
	requestCPUQuotaUsed := resource.Quantity{}
	limitCPUQuota := resource.Quantity{}
	limitCPUQuotaUsed := resource.Quantity{}
	cpuQuotas, err := globalK8sClient.Client.CoreV1().ResourceQuotas(pod.Namespace).List(metav1.ListOptions{})
	if err != nil {
		logrus.Error(err)
		return toAdmissionResponse(err)
	}
	for i := range cpuQuotas.Items {
		if request, found := cpuQuotas.Items[i].Spec.Hard[corev1.ResourceRequestsCPU]; found {
			requestCPUQuota.Add(request)
		}
		if request, found := cpuQuotas.Items[i].Status.Used[corev1.ResourceRequestsCPU]; found {
			requestCPUQuotaUsed.Add(request)
		}
		if limit, found := cpuQuotas.Items[i].Spec.Hard[corev1.ResourceRequestsCPU]; found {
			limitCPUQuota.Add(limit)
		}
		if limit, found := cpuQuotas.Items[i].Status.Used[corev1.ResourceRequestsCPU]; found {
			limitCPUQuotaUsed.Add(limit)
		}
	}
	if requestCPUQuota.IsZero() && limitCPUQuota.IsZero() {
		logrus.Infof("no GPU Quota defined for Namespece: %s", pod.Namespace)
		return &reviewResponse
	}

	var msg string
	requestCPU, limitCPU := calcPodCPUUsage(&pod)
	oldRequestGPU := resource.Quantity{}
	oldLimitGPU := resource.Quantity{}
	if isUpdate {
		oldRequestGPU, oldLimitGPU = calcPodCPUUsage(&oldPod)
	}

	requestCPU.Sub(oldRequestGPU)
	limitCPU.Sub(oldLimitGPU)

	requestCPUQuota.Sub(requestCPUQuotaUsed)
	limitCPUQuota.Sub(limitCPUQuotaUsed)
	if requestCPUQuota.Cmp(requestCPU) < 0 {
		requestCPUQuota.Add(requestCPUQuotaUsed)
		reviewResponse.Allowed = false
		msg = msg + fmt.Sprintf("exceeded quota: %s, requested(%s) + used(%s) > limited(%s)",
			corev1.ResourceRequestsCPU,
			requestCPU.String(),
			requestCPUQuotaUsed.String(),
			requestCPUQuota.String(),
		)
	}
	if limitCPUQuota.Cmp(limitCPU) < 0 {
		limitCPUQuota.Add(limitCPUQuotaUsed)
		reviewResponse.Allowed = false
		msg = msg + fmt.Sprintf("exceeded quota: %s, requested(%s) + used(%s) > limited(%s)",
			corev1.ResourceLimitsCPU,
			limitCPU.String(),
			limitCPUQuotaUsed.String(),
			limitCPUQuota.String(),
		)
	}
	if !reviewResponse.Allowed {
		reviewResponse.Result = &metav1.Status{Message: strings.TrimSpace(msg)}
	}
	return &reviewResponse
}

type admitFunc func(v1beta1.AdmissionReview) *v1beta1.AdmissionResponse

func serve(w http.ResponseWriter, r *http.Request, admit admitFunc) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		logrus.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	var reviewResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		logrus.Error(err)
		reviewResponse = toAdmissionResponse(err)
	} else {
		reviewResponse = admit(ar)
	}

	response := v1beta1.AdmissionReview{}
	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = ar.Request.UID
	}
	// reset the Object and OldObject, they are not needed in a response.
	ar.Request.Object = runtime.RawExtension{}
	ar.Request.OldObject = runtime.RawExtension{}

	resp, err := json.Marshal(response)
	if err != nil {
		logrus.Error(err)
	}
	if _, err := w.Write(resp); err != nil {
		logrus.Error(err)
	}
}

func ServePods(w http.ResponseWriter, r *http.Request) {
	serve(w, r, admitPod)
}