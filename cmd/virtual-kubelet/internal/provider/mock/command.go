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
	"bytes"
	"github.com/virtual-kubelet-cmd/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sync"
)

var home_dir = os.Getenv("HOME")
var start_container = metav1.NewTime(time.Now())

func (p *MockProvider) collectScripts(ctx context.Context, pod *v1.Pod, vol map[string]string) map[string][]string {
	// define a map to store the bash scripts, as the key is the container name, the value is the list of bash scripts
	bash_scripts := make(map[string][]string)
	for _, c := range pod.Spec.Containers {
		bash_scripts[c.Name] = []string{}
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
					log.G(ctx).Infof("found bash script %s", script)
					bash_scripts[c.Name] = append(bash_scripts[c.Name], script)
				}
			}
		}
	}
	return bash_scripts
}



// run scripts in parallel
func (p *MockProvider) runBashScriptParallel(ctx context.Context, pod *v1.Pod, vol map[string]string) {
	bash_scripts := p.collectScripts(ctx, pod, vol)
	var wg sync.WaitGroup
	for _, c := range pod.Spec.Containers {
		for _, script := range bash_scripts[c.Name] {
			wg.Add(1)
			go func(script string, c v1.Container) {
				defer wg.Done()
				_, err, leader_pid := runScript(ctx, script)
				if err != nil {
					log.G(ctx).Infof("failed to run script %s; error: %v", script, err)
					pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
						Name:         c.Name,
						Image:        c.Image,
						Ready:        false,
						RestartCount: 0,
						State: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{
								Message:    fmt.Sprintf("failed to run script %s; error: %v", script, err),
								FinishedAt: metav1.NewTime(time.Now()),
								Reason:     "ScriptRunFailed",
								StartedAt:  start_container,
							},
						},
					})
					return
				}

				//write the leader pid to the leader_pid file
				// name leader_pid file as the script name without .sh and add .leader_pid
				leader_pid_file := strings.TrimSuffix(script, ".sh") + ".leader_pid"
				err = ioutil.WriteFile(leader_pid_file, []byte(fmt.Sprintf("%v", leader_pid)), 0644)
				if err != nil {
					log.G(ctx).Infof("failed to write leader pid to file %s; error: %v", leader_pid_file, err)
					pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
						Name:         c.Name,
						Image:        c.Image,
						Ready:        false,
						RestartCount: 0,
						State: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{
								Message:    fmt.Sprintf("failed to write leader pid to file %s; error: %v", leader_pid_file, err),
								FinishedAt: metav1.NewTime(time.Now()),
								Reason:     "LeaderPidWriteFailed",
								StartedAt:  start_container,
							},
						},
					})
					return
				}
				log.G(ctx).Infof("successfully write leader pid to file %s", leader_pid_file)


				// update the container status to running
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
							StartedAt:  metav1.NewTime(time.Now()),
						},
					},
				})


			}(script, c)
		}
	}
	wg.Wait()
}





func runScript(ctx context.Context, scriptName string) (string, error, int) {
    cmd := exec.Command("bash", scriptName)

    var out bytes.Buffer
    var stderr bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &stderr

    err := cmd.Run()
    if err != nil {
        return "", err, 0
    }
	leader_pid := cmd.Process.Pid
    log.G(ctx).Infof("bash script name: %s", scriptName)
	log.G(ctx).Infof("leader pid: %v", leader_pid)

    return out.String(), nil, leader_pid
}



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