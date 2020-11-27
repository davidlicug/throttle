package util

import (
	"k8s.io/client-go/kubernetes"
	throttleClientSet "github.com/xychu/throttle/pkg/client/clientset/versioned"
)

type MemberOps struct {
	Username string		`json:username`
	Pk int				`json:pk`
	Email string		`json:email`
}

type QuotaUserOps struct{
	Id int				`json:id`
	Name string			`json:name`
	Members []MemberOps	`json:members`
}

type K8sClient struct {
	Client *kubernetes.Clientset
	ThrottleClient *throttleClientSet.Clientset

}
