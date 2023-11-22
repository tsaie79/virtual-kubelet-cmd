package mock

import (
	"os/exec"
	"path"
	"strings"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/virtual-kubelet-cmd/log"
	"fmt"
	"time"
	"context"
	
)


func (p *MockProvider) runBashScript(ctx context.Context, pod *v1.Pod, vol map[string]string) {
	for _, c := range pod.Spec.Containers {
		start_container := metav1.NewTime(time.Now())
		for _, volMount := range c.VolumeMounts {
			workdir := vol[volMount.Name]
			log.G(ctx).Infof("volume mount directory: %s", workdir)

			// if the volume mount is not found in the volume map, return error
			if workdir == "" {
				log.G(ctx).Infof("volume mount %s not found in the volume map", volMount.Name)
				// update the container status to failed
				pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
					Name:         c.Name,
					Image:        c.Image,
					Ready:        false,
					RestartCount: 0,
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message:    "volume mount not found in the volume map",
							FinishedAt: metav1.NewTime(time.Now()),
							Reason:     "VolumeMountNotFound",
							StartedAt:  start_container,
						},
					},
				})
				break
			}


			// run the command in the workdir
			//scan the workdir for bash scripts
			files, err := ioutil.ReadDir(workdir)
			if err != nil {
				log.G(ctx).Infof("failed to read workdir %s; error: %v", workdir, err)
				// update the container status to failed
				pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
					Name:         c.Name,
					Image:        c.Image,
					Ready:        false,
					RestartCount: 0,
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message:    fmt.Sprintf("failed to read workdir %s; error: %v", workdir, err),
							FinishedAt: metav1.NewTime(time.Now()),
							Reason:     "WorkdirReadFailed",
							StartedAt:  start_container,
						},
					},
				})
				break
			}


			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".sh") {
					script := path.Join(workdir, f.Name())
					log.G(ctx).Infof("running bash script %s", script)

					cmd := exec.CommandContext(ctx, "bash", script)
					err := cmd.Run()

					if err != nil {
						log.G(ctx).Infof("failed to run bash script: %s; error: %v", script, err)
						// update the container status to failed
						pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
							Name:         c.Name,
							Image:        c.Image,
							Ready:        false,
							RestartCount: 0,
							State: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									Message:    fmt.Sprintf("failed to run bash script: %s; error: %v", script, err),
									FinishedAt: metav1.NewTime(time.Now()),
									Reason:     "BashScriptFailed",
									StartedAt:  start_container,
								},
							},
						})
						break	
					}

					log.G(ctx).Infof("bash script executed successfully in workdir %s", script)
					// update the container status to success
					pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
						Name:         c.Name,
						Image:        c.Image,
						Ready:        true,
						RestartCount: 0,
						State: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{
								Message:    fmt.Sprintf("bash script executed successfully in workdir %s", script),
								FinishedAt: metav1.NewTime(time.Now()),
								Reason:     "CmdSucceeded",
								StartedAt:  start_container,
							},
						},
					})

					sleep := time.Duration(10) * time.Second
					time.Sleep(sleep)
				}
			}
		}
	}
}