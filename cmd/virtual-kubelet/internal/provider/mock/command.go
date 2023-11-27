package mock

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"strings"
	"time"
	"os"
	"github.com/virtual-kubelet-cmd/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
var home_dir = os.Getenv("HOME")

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
				start_running := metav1.NewTime(time.Now())
				if strings.HasSuffix(f.Name(), ".sh") {
					script := path.Join(workdir, f.Name())
					log.G(ctx).Infof("running bash script %s", script)

					// run the bash script in the workdir
					leader_pid, err := executeProcess(ctx, script)
					if err != nil {
						log.G(ctx).Infof("failed to run bash script: %s; error: %v", script, err)
					}
					log.G(ctx).Infof("Leader pid: %v", leader_pid)
				

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
									Reason:     "BashScriptFailed",
									StartedAt:  start_running,
								},
							},
						})
						break
					}


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
									Reason:     "BashScriptFailed",
									StartedAt:  start_running,
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
								Reason:     "BashScriptSuccess",
								StartedAt:  start_running,
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


// run the bash script in the workdir and keep track of the pids of the processes and their children
func executeProcess(ctx context.Context, script string) (int, error) {
	// dont wait for the script to finish
	cmd := exec.CommandContext(ctx, "bash", script) // run the bash script in the workdir without waiting for it to finish
	err := cmd.Start()


	if err != nil {
		return 0, err
	}

	leader_pid := cmd.Process.Pid
	return leader_pid, nil
}