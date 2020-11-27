package util

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"strconv"
	"gopkg.in/gomail.v2"
	"github.com/sirupsen/logrus"

	"github.com/xychu/throttle/cmd/app/options"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	throttle_clientset "github.com/xychu/throttle/pkg/client/clientset/versioned"
)

func MailUsers(url string)([]string, error){
	if url == "" {
		return nil, fmt.Errorf("invalid string url:%s", url)
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var qu QuotaUserOps
	if err := json.Unmarshal(data, &qu); err != nil {
		return nil, fmt.Errorf("json unmarshal error:%s", err)
	}

	var userEmails []string
	for _, m := range qu.Members {
		if m.Email == "" {
			continue
		}
		userEmails = append(userEmails, m.Email)
	}

	return userEmails, nil
}


func SendMail(mailTo []string, subject string, body string) error {
	mailConn := map[string]string{
		"user": "9n-cloud@jd.com",
		"pass": "r4r3St*****7a7Uk",//TMubgdxyast@771
		"host": "smtp-server.jd.local",
		"port": "32000",
	}

	port, _ := strconv.Atoi(mailConn["port"]) //转换端口类型为int

	m := gomail.NewMessage()

	m.SetHeader("From",  m.FormatAddress(mailConn["user"], "9n-cloud")) //这种方式可以添加别名，即“XX官方”
	//说明：如果是用网易邮箱账号发送，以下方法别名可以是中文，如果是qq企业邮箱，以下方法用中文别名，会报错，需要用上面此方法转码
	//m.SetHeader("From", "FB Sample"+"<"+mailConn["user"]+">") //这种方式可以添加别名，即“FB Sample”， 也可以直接用<code>m.SetHeader("From",mailConn["user"])</code> 读者可以自行实验下效果
	//m.SetHeader("From", mailConn["user"])
	m.SetHeader("To", mailTo...)    //发送给多个用户
	m.SetHeader("Subject", subject) //设置邮件主题
	m.SetBody("text/html", body)    //设置邮件正文

	d := gomail.NewDialer(mailConn["host"], port, mailConn["user"], mailConn["pass"])
	err := d.DialAndSend(m)
	return err
}


func NewK8sClient(opt *options.ServerOption)(*K8sClient, error){
	var client K8sClient
	config, err := clientcmd.BuildConfigFromFlags(opt.MasterURL, opt.Kubeconfig)
	if err != nil {
		logrus.Errorf("build config from flag error: %s",err)
		return nil,err
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Errorf("kubernetes new client for config error: %s",err)
		return nil,err
	}

	client.Client  = clientset

	if opt.K8sLowerVersion {
		throttleClientset, err := throttle_clientset.NewForConfig(config)
		if err != nil {
			logrus.Errorf("kubernetes new throttle client for config error: %s",err)
			return nil, err
		}
		client.ThrottleClient = throttleClientset
	}

	return &client, nil
}

func RemoveRepByMap(slc []string) []string {
	result := []string{}         		//存放返回的不重复切片
	tempMap := map[string]byte{} 		// 存放不重复主键
	for _, e := range slc {
		l := len(tempMap)
		tempMap[e] = 0 					//当e存在于tempMap中时，再次添加是添加不进去的，，因为key不允许重复
		//如果上一行添加成功，那么长度发生变化且此时元素一定不重复
		if len(tempMap) != l { 			// 加入map后，map长度变化，则元素不重复
			result = append(result, e) 	//当元素不重复时，将元素添加到切片result中
		}
	}
	return result
}

func IsNsInWhiteList(ns string, namespaces []string)bool{
	for _, namespace := range namespaces {
		if ns == namespace {
			return true
		}
	}

	return false
}
