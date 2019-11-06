/*
Copyright The Kmodules Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package v1beta1

import (
	"github.com/golang/glog"
	"github.com/pkg/errors"
	rbac "k8s.io/api/rbac/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kutil "kmodules.xyz/client-go"
)

func CreateOrPatchClusterRoleBinding(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*rbac.ClusterRoleBinding) *rbac.ClusterRoleBinding) (*rbac.ClusterRoleBinding, kutil.VerbType, error) {
	cur, err := c.RbacV1beta1().ClusterRoleBindings().Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating ClusterRoleBinding %s.", meta.Name)
		out, err := c.RbacV1beta1().ClusterRoleBindings().Create(transform(&rbac.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterRoleBinding",
				APIVersion: rbac.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchClusterRoleBinding(c, cur, transform)
}

func PatchClusterRoleBinding(c kubernetes.Interface, cur *rbac.ClusterRoleBinding, transform func(*rbac.ClusterRoleBinding) *rbac.ClusterRoleBinding) (*rbac.ClusterRoleBinding, kutil.VerbType, error) {
	return PatchClusterRoleBindingObject(c, cur, transform(cur.DeepCopy()))
}

func PatchClusterRoleBindingObject(c kubernetes.Interface, cur, mod *rbac.ClusterRoleBinding) (*rbac.ClusterRoleBinding, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, rbac.ClusterRoleBinding{})
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching ClusterRoleBinding %s with %s.", cur.Name, string(patch))
	out, err := c.RbacV1beta1().ClusterRoleBindings().Patch(cur.Name, types.StrategicMergePatchType, patch)
	return out, kutil.VerbPatched, err
}

func TryUpdateClusterRoleBinding(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*rbac.ClusterRoleBinding) *rbac.ClusterRoleBinding) (result *rbac.ClusterRoleBinding, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.RbacV1beta1().ClusterRoleBindings().Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.RbacV1beta1().ClusterRoleBindings().Update(transform(cur.DeepCopy()))
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update ClusterRoleBinding %s due to %v.", attempt, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = errors.Errorf("failed to update Role %s after %d attempts due to %v", meta.Name, attempt, err)
	}
	return
}
