/*
This file includes software developed at
Virtual Kubelet (https://github.com/virtual-kubelet/virtual-kubelet)

Copyright 2023 The Virtual Kubelet Authors

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


package mock

import (
	// "fmt"
	// "os/user"
	// "strconv"

	v1 "k8s.io/api/core/v1"
)

// uidGidFromSecurityContext returns the uid and gid (as a string) from the SecurityContext.
// If windowsOptions are set, the uid and gid *names* found there are returned. If these users
// can not be found on the system an error is returned. Every user must have a primary group, so
// either uid and gid or both empty or both set.
//
// If the uid is found, but gid is not, the primary group for uid is searched and returned.
// If no securityContext is found this returns the empty strings for uid and gid.
func uidGidFromSecurityContext(pod *v1.Pod, maproot int) (uid, gid string, err error) {
	if pod.Spec.SecurityContext == nil {
		return "", "", nil
	}
	// u := &user.User{}
	// s := pod.Spec.SecurityContext
	// if s.RunAsUser != nil {
	// 	uid = strconv.FormatInt(*s.RunAsUser, 10)
	// 	u, err = user.LookupId(uid)
	// 	if err != nil {
	// 		return "", "", err
	// 	}
	// }
	// if s.RunAsGroup != nil {
	// 	gid = strconv.FormatInt(*s.RunAsGroup, 10)
	// }
	// if s.WindowsOptions != nil {
	// 	if s.WindowsOptions.RunAsUserName != nil {
	// 		uid = *s.WindowsOptions.RunAsUserName
	// 		u, err = user.Lookup(uid)
	// 		if err != nil {
	// 			return "", "", err
	// 		}
	// 		uid = u.Uid
	// 	}
	// }

	// // if uid is set, but gid isn't, return the primary group for uid.
	// if uid != "" && gid == "" {
	// 	if u != nil {
	// 		gid = u.Gid
	// 	}
	// }

	// // Check if maproot is set and convert.
	// if uid == "0" && maproot > 0 {
	// 	mapuid := strconv.FormatInt(int64(maproot), 10)
	// 	u, err = user.LookupId(mapuid)
	// 	if err != nil {
	// 		return "", "", fmt.Errorf("root override UID %q, not found: %s", mapuid, err)
	// 	}
	// 	uid = u.Uid
	// 	gid = u.Gid
	// }

	return uid, gid, nil
}