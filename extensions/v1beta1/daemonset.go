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
	extensions "k8s.io/api/extensions/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kutil "kmodules.xyz/client-go"
)

func CreateOrPatchDaemonSet(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*extensions.DaemonSet) *extensions.DaemonSet) (*extensions.DaemonSet, kutil.VerbType, error) {
	cur, err := c.ExtensionsV1beta1().DaemonSets(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating DaemonSet %s/%s.", meta.Namespace, meta.Name)
		out, err := c.ExtensionsV1beta1().DaemonSets(meta.Namespace).Create(transform(&extensions.DaemonSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "DaemonSet",
				APIVersion: extensions.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchDaemonSet(c, cur, transform)
}

func PatchDaemonSet(c kubernetes.Interface, cur *extensions.DaemonSet, transform func(*extensions.DaemonSet) *extensions.DaemonSet) (*extensions.DaemonSet, kutil.VerbType, error) {
	return PatchDaemonSetObject(c, cur, transform(cur.DeepCopy()))
}

func PatchDaemonSetObject(c kubernetes.Interface, cur, mod *extensions.DaemonSet) (*extensions.DaemonSet, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, extensions.DaemonSet{})
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching DaemonSet %s/%s with %s.", cur.Namespace, cur.Name, string(patch))
	out, err := c.ExtensionsV1beta1().DaemonSets(cur.Namespace).Patch(cur.Name, types.StrategicMergePatchType, patch)
	return out, kutil.VerbPatched, err
}

func TryUpdateDaemonSet(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*extensions.DaemonSet) *extensions.DaemonSet) (result *extensions.DaemonSet, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.ExtensionsV1beta1().DaemonSets(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.ExtensionsV1beta1().DaemonSets(cur.Namespace).Update(transform(cur.DeepCopy()))
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update DaemonSet %s/%s due to %v.", attempt, cur.Namespace, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = errors.Errorf("failed to update DaemonSet %s/%s after %d attempts due to %v", meta.Namespace, meta.Name, attempt, err)
	}
	return
}

func WaitUntilDaemonSetReady(kubeClient kubernetes.Interface, meta metav1.ObjectMeta) error {
	return wait.PollImmediate(kutil.RetryInterval, kutil.ReadinessTimeout, func() (bool, error) {
		if obj, err := kubeClient.ExtensionsV1beta1().DaemonSets(meta.Namespace).Get(meta.Name, metav1.GetOptions{}); err == nil {
			return obj.Status.DesiredNumberScheduled == obj.Status.NumberReady, nil
		}
		return false, nil
	})
}
