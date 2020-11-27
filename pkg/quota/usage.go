package quota

import (
	"fmt"

	"net/http"
	"io/ioutil"
	"encoding/json"
	"strconv"
	"net/url"
)

const (
	reSyncResourceUsagePeriod = 24 * 7
	httpNoData = "http request no return data"
	httpRetDataCount = 120
)

var (
	//namespace -> cpu usage
	cpuUsageMap = make(map[string][reSyncResourceUsagePeriod]float64)
	//namespace -> memory usage
	memoryUsageMap = make(map[string][reSyncResourceUsagePeriod]float64)
	//namespace -> gpu usage
	gpuUsageMap = make(map[string][reSyncResourceUsagePeriod]float64)

	cpuAvgUsageOneWeek = make(map[string]float64)
	memoryAvgUsageOneWeek = make(map[string]float64)
	gpuAvgUsageOneWeek = make(map[string]float64)
)

type PrometheusResult struct {
	Status string					`json:"status"`
	Data DataOps 					`json:"data"`
}

type DataOps struct {
	ResultType string				`json:"resultType"`
	Result []ResultOps				`json:"result"`
}

type ResultOps struct {
	Metric MetricOps				`json:"metric"`
	Values [][2]interface{}			        `json:"values"`
}

type MetricOps struct {
	Namespace string				`json:"namespace"`
}

func calcNsAvgUsage(urlStr string)(float64, error){
	var avgUsage float64
	client := &http.Client{}
	u, _ := url.Parse(urlStr)
	q := u.Query()
	u.RawQuery = q.Encode()
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return avgUsage, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return avgUsage, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return avgUsage, err
	}

	var pr PrometheusResult
	if err = json.Unmarshal(data, &pr); err != nil {
		return avgUsage, err
	}

	if len(pr.Data.Result) < 1 || len(pr.Data.Result[0].Values) < 1 {
		return avgUsage, fmt.Errorf("%s", httpNoData)
	}

	avgUsage, err = calcAvgUsage(pr)

	return avgUsage, err
}

func calcAvgUsage(pr PrometheusResult)(float64, error){
	var (
		sumUsage float64
		avgUsage float64
		count int
	)
	for _, result := range pr.Data.Result {
		if len(result.Values) < httpRetDataCount {
			return avgUsage, fmt.Errorf("No avaliable data must great 120. " )
		}
		for _, value := range result.Values {
			if v, ok := value[1].(string); ok {
				n, _ := strconv.ParseFloat(v, 64)
				sumUsage += n
				count++
			}
		}
		if count > 0  {
			avgUsage = sumUsage/float64(count)
		}
		break
	}

	return avgUsage, nil
}