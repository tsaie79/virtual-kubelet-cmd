package mock

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// var home_dir = os.Getenv("HOME")
var start_container = metav1.NewTime(time.Now())
var home_dir = os.Getenv("HOME")

func (p *MockProvider) collectScripts(ctx context.Context, pod *v1.Pod, vol map[string]string) {
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

			log.G(ctx).Infof("volumeMount: %s, volume mount directory: %s", volMount.Name, mountdir)
			// if the volume mount is not found in the volume map, return error
			if workdir == "" {
				log.G(ctx).Infof("volumeMount %s not found in the volume map", volMount.Name)
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
				err := moveFile(ctx, workdir, mountdir, f.Name())
				if err != nil {
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
}

// run container in parallel
func (p *MockProvider) runScriptParallel(ctx context.Context, pod *v1.Pod, vol map[string]string) (chan error, chan v1.ContainerStatus) {
	p.collectScripts(ctx, pod, vol)

	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	cstatusChan := make(chan v1.ContainerStatus, 1)
	for _, c := range pod.Spec.Containers {

		wg.Add(1)
		go func(c v1.Container) {
			defer wg.Done()
			log.G(ctx).Infof("---------------container name: %s", c.Name)
			var leader_pid_volmount string
			for _, volMount := range c.VolumeMounts {
				if strings.Contains(volMount.Name, "leader-pid") {
					leader_pid_volmount = volMount.MountPath
				}
			}
			if leader_pid_volmount == "" {
				log.G(ctx).Infof("leader pid volume mount not found for container %s", c.Name)
				return
			}
			//define command to run the bash script based on c.Command of list of strings
			var command = c.Command
			if len(command) == 0 {
				log.G(ctx).Infof("no command found for container %s", c.Name)
				return
			}

			var args string
			if len(c.Args) == 0 {
				log.G(ctx).Infof("no args found for container %s", c.Name)
				return
			} else {
				args = strings.Join(c.Args, " ")
				// if args contains "~", replace it with home_dir
				args = strings.Replace(args, "~", home_dir, 1)
			}

			env := c.Env // what is the type of c.Env? []v1.EnvVar
			_, leader_pid, err := runScript(ctx, command, args, env)
			// print pod.status.containerStatuses
			if err != nil {
				// report error to errChan
				errChan <- err
				// report container status to container_status
				cstatusChan <- v1.ContainerStatus{
					Name:         c.Name,
					Image:        c.Image,
					Ready:        false,
					RestartCount: 0,
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message:    fmt.Sprintf("failed to run cmd %s, arg %s; error: %v", command, args, err),
							FinishedAt: metav1.NewTime(time.Now()),
							Reason:     "RunScriptFailed",
							StartedAt:  start_container,
						},
					},
				}
				return
			}

			//write the leader pid to the leader_pid file
			// name leader_pid file as container name + .leader_pid
			leader_pid_file := path.Join(c.Name + ".leader_pid")
			err = writeLeaderPid(ctx, leader_pid_volmount, leader_pid_file, leader_pid)
			if err != nil {
				//report error to errChan
				errChan <- err
				// report container status to container_status
				cstatusChan <- v1.ContainerStatus{
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
				}
				return
			}

			// update the container status to success
			cstatusChan <- v1.ContainerStatus{
				Name:         c.Name,
				Image:        c.Image,
				Ready:        true,
				RestartCount: 0,
				State: v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						Message:    fmt.Sprintf("container %s executed successfully", c.Name),
						Reason:     "ContainerRunSuccess",
						FinishedAt: metav1.NewTime(time.Now()),
						StartedAt:  start_container,
					},
				},
			}
		}(c)
	}

	go func() {
		wg.Wait()
		close(errChan)
		close(cstatusChan) // close the channel
	}()

	return errChan, cstatusChan

	// // update container status based on the output of the goroutines above
	// for err := range errChan {
	// 	if err != nil {
	// 		log.G(ctx).Infof("error: %v", err)
	// 		// update the container status to failed
	// 	}
	// }

	// for cstatus := range cstatusChan {
	// 	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, cstatus)
	// 	log.G(ctx).Infof("container status: %v", cstatus)
	// }
}

func writeLeaderPid(ctx context.Context, leader_pid_volmount string, leader_pid_file string, leader_pid int) error {
	leader_pid_volmount = strings.Replace(leader_pid_volmount, "~", home_dir, 1)
	//write the leader pid to the leader_pid file
	// name leader_pid file as container name + .leader_pid
	leader_pid_file = path.Join(leader_pid_volmount, leader_pid_file)
	err := ioutil.WriteFile(leader_pid_file, []byte(fmt.Sprintf("%v", leader_pid)), 0644)
	if err != nil {
		log.G(ctx).Infof("failed to write leader pid to file %s; error: %v", leader_pid_file, err)
		return err
	}
	log.G(ctx).Infof("successfully wrote leader pid to file %s", leader_pid_file)
	return nil
}

func runScript(ctx context.Context, command []string, args string, env []v1.EnvVar) (string, int, error) {
	// run the script with the env variables set
	// run the command like [command[0], command[1], ...] args
	command = append(command, args)
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = os.Environ()
	for _, e := range env {
		log.G(ctx).Infof("env name: %s, env value: %s", e.Name, e.Value)
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", e.Name, e.Value))
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	log.G(ctx).Infof("start running command: %s, arg: %s", command, args)

	err := cmd.Run()
	if err != nil {
		log.G(ctx).Infof("failed to run the cmd. error: %v", err)
		return "", 0, err
	}
	leader_pid := cmd.Process.Pid
	log.G(ctx).Infof("successfully ran cmd pid: %v, stdout: %s, stderr: %s", leader_pid, out.String(), stderr.String())
	return out.String(), leader_pid, nil
}

func moveFile(ctx context.Context, src string, dst string, filename string) error {
	// create the destination directory if it does not exist
	err := exec.Command("mkdir", "-p", dst).Run()
	if err != nil {
		log.G(ctx).Infof("failed to create directory %s; error: %v", dst, err)
		return err
	}
	//mv the file to the destination directory
	err = exec.Command("mv", path.Join(src, filename), path.Join(dst, filename)).Run()
	if err != nil {
		log.G(ctx).Infof("failed to move file %s to %s; error: %v", path.Join(src, filename), path.Join(dst, filename), err)
		return err
	}
	return nil
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
