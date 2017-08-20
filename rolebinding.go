package kutil

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/mattbaird/jsonpatch"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	rbac "k8s.io/client-go/pkg/apis/rbac/v1beta1"
)

func CreateOrPatchRoleBinding(c clientset.Interface, obj *rbac.RoleBinding) (*rbac.RoleBinding, error) {
	cur, err := c.RbacV1beta1().RoleBindings(obj.Namespace).Get(obj.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		return c.RbacV1beta1().RoleBindings(obj.Namespace).Create(obj)
	} else if err != nil {
		return nil, err
	}
	return PatchRoleBinding(c, cur, func(*rbac.RoleBinding) *rbac.RoleBinding { return obj })
}

func PatchRoleBinding(c clientset.Interface, cur *rbac.RoleBinding, transform func(*rbac.RoleBinding) *rbac.RoleBinding) (*rbac.RoleBinding, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, err
	}

	modJson, err := json.Marshal(transform(cur))
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.CreatePatch(curJson, modJson)
	if err != nil {
		return nil, err
	}
	pb, err := json.MarshalIndent(patch, "", "  ")
	if err != nil {
		return nil, err
	}
	glog.V(5).Infof("Patching RoleBinding %s@%s with %s.", cur.Name, cur.Namespace, string(pb))
	return c.RbacV1beta1().RoleBindings(cur.Namespace).Patch(cur.Name, types.JSONPatchType, pb)
}

func TryPatchRoleBinding(c clientset.Interface, meta metav1.ObjectMeta, transform func(*rbac.RoleBinding) *rbac.RoleBinding) (*rbac.RoleBinding, error) {
	attempt := 0
	for ; attempt < maxAttempts; attempt = attempt + 1 {
		cur, err := c.RbacV1beta1().RoleBindings(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(err) {
			return cur, err
		} else if err == nil {
			return PatchRoleBinding(c, cur, transform)
		}
		glog.Errorf("Attempt %d failed to patch RoleBinding %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(retryInterval)
	}
	return nil, fmt.Errorf("Failed to patch RoleBinding %s@%s after %d attempts.", meta.Name, meta.Namespace, attempt)
}

func TryUpdateRoleBinding(c clientset.Interface, meta metav1.ObjectMeta, transform func(*rbac.RoleBinding) *rbac.RoleBinding) (*rbac.RoleBinding, error) {
	attempt := 0
	for ; attempt < maxAttempts; attempt = attempt + 1 {
		cur, err := c.RbacV1beta1().RoleBindings(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(err) {
			return cur, err
		} else if err == nil {
			return c.RbacV1beta1().RoleBindings(cur.Namespace).Update(transform(cur))
		}
		glog.Errorf("Attempt %d failed to update RoleBinding %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(retryInterval)
	}
	return nil, fmt.Errorf("Failed to update RoleBinding %s@%s after %d attempts.", meta.Name, meta.Namespace, attempt)
}
