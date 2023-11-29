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

// var home_dir = os.Getenv("HOME")
var start_container = metav1.NewTime(time.Now())
var home_dir = os.Getenv("HOME")


func (p *MockProvider) collectScripts(ctx context.Context, pod *v1.Pod, vol map[string]string) map[string][]string {
	// define a map to store the bash scripts, as the key is the container name, the value is the list of bash scripts
	scripts := make(map[string][]string)
	for _, c := range pod.Spec.Containers {
		log.G(ctx).Infof("container name: %s", c.Name)
		scripts[c.Name] = []string{}
		for _, volMount := range c.VolumeMounts {
			workdir := vol[volMount.Name]
			mountdir := volMount.MountPath
			// if mountdir has ~, replace it with home_dir
			if strings.HasPrefix(mountdir, "~") {
				mountdir = strings.Replace(mountdir, "~", home_dir, 1)
			}

			log.G(ctx).Infof("volume name: %s, volume mount directory: %s", volMount.Name, mountdir)
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
				continue
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
				continue
			}

			for _, f := range files {
				log.G(ctx).Infof("file name: %s", f.Name())
				// if f.Name() contains crt, key, or pem, skip it
				if strings.Contains(f.Name(), "crt") || strings.Contains(f.Name(), "key") || strings.Contains(f.Name(), "pem") {
					log.G(ctx).Infof("file name %s contains crt, key, or pem, skip it", f.Name())
					continue
				}
				
				// move f to the volume mount directory
				err := exec.Command("mv", path.Join(workdir, f.Name()), path.Join(mountdir, f.Name())).Run()
				if err != nil {
					log.G(ctx).Infof("failed to move file %s to %s; error: %v", path.Join(workdir, f.Name()), path.Join(mountdir, f.Name()), err)
					pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
						Name:         c.Name,
						Image:        c.Image,
						Ready:        false,
						RestartCount: 0,
						State: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{
								Message:    fmt.Sprintf("failed to move file %s to %s; error: %v", path.Join(workdir, f.Name()), path.Join(mountdir, f.Name()), err),
								FinishedAt: metav1.NewTime(time.Now()),
								Reason:     "FileMoveFailed",
								StartedAt:  start_container,
							},
						},
					})
					continue
				}
				
				script := path.Join(mountdir, f.Name())
				log.G(ctx).Infof("found script %s", script)
				scripts[c.Name] = append(scripts[c.Name], script)
				// if strings.HasSuffix(f.Name(), ".job") {
				// 	script := path.Join(mountdir, f.Name())
				// 	log.G(ctx).Infof("found job script %s", script)
				// 	job_scripts[c.Name] = append(job_scripts[c.Name], script)
				// } else {
				// 	log.G(ctx).Infof("found non-job script %s", f.Name())
				// }
			}
		}
	}
	log.G(ctx).Infof("found scripts: %v", scripts)
	return scripts
}

// run scripts in parallel
func (p *MockProvider) runScriptParallel(ctx context.Context, pod *v1.Pod, vol map[string]string) {
	scripts := p.collectScripts(ctx, pod, vol)

	var wg sync.WaitGroup
	for _, c := range pod.Spec.Containers {

		if len(scripts[c.Name]) == 0 {
			log.G(ctx).Infof("no scripts found for container %s", c.Name)
			continue
		}
		
		//define command to run the bash script based on c.Command of list of strings
		var command string
		if len(c.Command) == 0 {
			log.G(ctx).Infof("no command found for container %s", c.Name)
			continue
		} else {
			command = strings.Join(c.Command, " ")
		}

		for _, script := range scripts[c.Name] {
			wg.Add(1)
			go func(script string, c v1.Container) {
				defer wg.Done()
				env := c.Env // what is the type of c.Env? []v1.EnvVar
				_, leader_pid, err := runScript(ctx, script, command, env)
				if err != nil {
					log.G(ctx).Infof("failed to run job script %s; error: %v", script, err)
					pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
						Name:         c.Name,
						Image:        c.Image,
						Ready:        false,
						RestartCount: 0,
						State: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{
								Message:    fmt.Sprintf("failed to run job script %s; error: %v", script, err),
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
				leader_pid_file := strings.TrimSuffix(script, ".job") + ".leader_pid"
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
				log.G(ctx).Infof("job script executed successfully in workdir %s", script)
				// update the container status to success
				pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
					Name:         c.Name,
					Image:        c.Image,
					Ready:        true,
					RestartCount: 0,
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message:   fmt.Sprintf("job script executed successfully in workdir %s", script),
							Reason:    "ScriptRunSuccess",
							StartedAt: metav1.NewTime(time.Now()),
						},
					},
				})

			}(script, c)
		}
	}
	wg.Wait()

	// no job scripts found
	if len(pod.Status.ContainerStatuses) == 0 {
		log.G(ctx).Infof("no job scripts found")
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
			Name:         pod.Spec.Containers[0].Name,
			Image:        pod.Spec.Containers[0].Image,
			Ready:        false,
			RestartCount: 0,
			State: v1.ContainerState{
				Terminated: &v1.ContainerStateTerminated{
					Message:    "no job scripts found",
					FinishedAt: metav1.NewTime(time.Now()),
					Reason:     "NoJobScriptFound",
					StartedAt:  start_container,
				},
			},
		})
	}

}

func runScript(ctx context.Context, scriptName string, command string, env []v1.EnvVar) (string, int, error) {
	// run the script with the env variables set
	cmd := exec.Command(command, scriptName)
	cmd.Env = os.Environ()
	for _, e := range env {
		log.G(ctx).Infof("env name: %s, env value: %s", e.Name, e.Value)
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", e.Name, e.Value))
	}
	
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", 0, err
	}
	leader_pid := cmd.Process.Pid
	log.G(ctx).Infof("command: %s, script: %s, stdout: %s, stderr: %s", command, scriptName, out.String(), stderr.String())
	log.G(ctx).Infof("leader pid: %v", leader_pid)

	return out.String(), leader_pid, nil
}

// func (p *MockProvider) runBashScript(ctx context.Context, pod *v1.Pod, vol map[string]string) {
// 	for _, c := range pod.Spec.Containers {
// 		start_container := metav1.NewTime(time.Now())
// 		for _, volMount := range c.VolumeMounts {
// 			workdir := vol[volMount.Name]
// 			log.G(ctx).Infof("volume mount directory: %s", workdir)

// 			// if the volume mount is not found in the volume map, return error
// 			if workdir == "" {
// 				log.G(ctx).Infof("volume mount %s not found in the volume map", volMount.Name)
// 				// update the container status to failed
// 				pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
// 					Name:         c.Name,
// 					Image:        c.Image,
// 					Ready:        false,
// 					RestartCount: 0,
// 					State: v1.ContainerState{
// 						Terminated: &v1.ContainerStateTerminated{
// 							Message:    "volume mount not found in the volume map",
// 							FinishedAt: metav1.NewTime(time.Now()),
// 							Reason:     "VolumeMountNotFound",
// 							StartedAt:  start_container,
// 						},
// 					},
// 				})
// 				break
// 			}

// 			// run the command in the workdir
// 			//scan the workdir for bash scripts
// 			files, err := ioutil.ReadDir(workdir)
// 			if err != nil {
// 				log.G(ctx).Infof("failed to read workdir %s; error: %v", workdir, err)
// 				// update the container status to failed
// 				pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
// 					Name:         c.Name,
// 					Image:        c.Image,
// 					Ready:        false,
// 					RestartCount: 0,
// 					State: v1.ContainerState{
// 						Terminated: &v1.ContainerStateTerminated{
// 							Message:    fmt.Sprintf("failed to read workdir %s; error: %v", workdir, err),
// 							FinishedAt: metav1.NewTime(time.Now()),
// 							Reason:     "WorkdirReadFailed",
// 							StartedAt:  start_container,
// 						},
// 					},
// 				})
// 				break
// 			}

// 			for _, f := range files {
// 				start_running := metav1.NewTime(time.Now())
// 				if strings.HasSuffix(f.Name(), ".job") {
// 					script := path.Join(workdir, f.Name())
// 					log.G(ctx).Infof("running bash script %s", script)

// 					// run the bash script in the workdir
// 					leader_pid, err := executeProcess(ctx, script)
// 					if err != nil {
// 						log.G(ctx).Infof("failed to run bash script: %s; error: %v", script, err)
// 					}
// 					log.G(ctx).Infof("Leader pid: %v", leader_pid)

// 					if err != nil {
// 						log.G(ctx).Infof("failed to run bash script: %s; error: %v", script, err)
// 						// update the container status to failed
// 						pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
// 							Name:         c.Name,
// 							Image:        c.Image,
// 							Ready:        false,
// 							RestartCount: 0,
// 							State: v1.ContainerState{
// 								Terminated: &v1.ContainerStateTerminated{
// 									Message:    fmt.Sprintf("failed to run bash script: %s; error: %v", script, err),
// 									Reason:     "BashScriptFailed",
// 									StartedAt:  start_running,
// 								},
// 							},
// 						})
// 						break
// 					}

// 					if err != nil {
// 						log.G(ctx).Infof("failed to run bash script: %s; error: %v", script, err)
// 						// update the container status to failed
// 						pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
// 							Name:         c.Name,
// 							Image:        c.Image,
// 							Ready:        false,
// 							RestartCount: 0,
// 							State: v1.ContainerState{
// 								Terminated: &v1.ContainerStateTerminated{
// 									Message:    fmt.Sprintf("failed to run bash script: %s; error: %v", script, err),
// 									Reason:     "BashScriptFailed",
// 									StartedAt:  start_running,
// 								},
// 							},
// 						})
// 						break
// 					}

// 					log.G(ctx).Infof("bash script executed successfully in workdir %s", script)
// 					// update the container status to success
// 					pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
// 						Name:         c.Name,
// 						Image:        c.Image,
// 						Ready:        true,
// 						RestartCount: 0,
// 						State: v1.ContainerState{
// 							Terminated: &v1.ContainerStateTerminated{
// 								Message:    fmt.Sprintf("bash script executed successfully in workdir %s", script),
// 								Reason:     "BashScriptSuccess",
// 								StartedAt:  start_running,
// 							},
// 						},
// 					})

// 					sleep := time.Duration(10) * time.Second
// 					time.Sleep(sleep)
// 				}
// 			}
// 		}
// 	}
// }

// // run the bash script in the workdir and keep track of the pids of the processes and their children
// func executeProcess(ctx context.Context, script string) (int, error) {
// 	// dont wait for the script to finish
// 	cmd := exec.CommandContext(ctx, "bash", script) // run the bash script in the workdir without waiting for it to finish
// 	err := cmd.Start()

// 	if err != nil {
// 		return 0, err
// 	}

// 	leader_pid := cmd.Process.Pid
// 	return leader_pid, nil
// }
