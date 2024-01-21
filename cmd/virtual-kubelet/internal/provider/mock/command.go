package mock

import (
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
	"syscall"
)

func newCollectScripts(ctx context.Context, c *v1.Container, podName string, vol map[string]string) (map[string]string, *v1.ContainerState, error) {
	timeStart := metav1.NewTime(time.Now())
	// define a map to store the bash scripts, as the key is the container name, the value is the list of bash scripts
	var (
		scriptMap = make(map[string]string)
		containerState *v1.ContainerState
	)

	for _, volMount := range c.VolumeMounts {
		defaultVolumeDir := vol[volMount.Name]
		mountDir := path.Join(os.Getenv("HOME"), podName, "containers", volMount.MountPath)
		log.G(ctx).WithField("volume name", volMount.Name).WithField("mount directory", mountDir).Info("volumeMount")

		// run the command in the workdir
		//scan the workdir for bash scripts
		files, err := ioutil.ReadDir(defaultVolumeDir)
		if err != nil {
			log.G(ctx).WithField("default volume directory", defaultVolumeDir).Errorf("failed to read default volume directory; error: %v", err)
			
			containerState = &v1.ContainerState{
				Terminated: &v1.ContainerStateTerminated{
					Message:    fmt.Sprintf("failed to read default volume directory %s; error: %v", defaultVolumeDir, err),
					FinishedAt: metav1.NewTime(time.Now()),
					Reason:     "ContainerCreatingError",
					StartedAt:  timeStart,
				},
			}
			return nil, containerState, err
		}

		for _, f := range files {
			log.G(ctx).WithField("File name", f.Name()).Info("File in default volume directory")

			// if f.Name() contains crt, key, or pem, skip it
			if strings.Contains(f.Name(), "crt") || strings.Contains(f.Name(), "key") || strings.Contains(f.Name(), "pem") {
				log.G(ctx).WithField("file_name", f.Name()).Info("file name contains crt, key, or pem, skip it")
				continue
			}

			// move f to the volume mount directory
			err := copyFile(ctx, defaultVolumeDir, mountDir, f.Name())
			if err != nil {
				log.G(ctx).WithField("File name", f.Name()).Errorf("failed to copy file; error: %v", err)

				containerState = &v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						Message:    fmt.Sprintf("failed to copy file %s to %s; error: %v", path.Join(defaultVolumeDir, f.Name()), path.Join(mountDir, f.Name()), err),
						FinishedAt: metav1.NewTime(time.Now()),
						Reason:     "ContainerCreatingError",
						StartedAt:  timeStart,
					},
				}
				return nil, containerState, err
			}
			scriptPath := path.Join(mountDir, f.Name())
			scriptMap[volMount.Name] = scriptPath
		}
	}
	return scriptMap, nil, nil
}

	
func (p *MockProvider) runScriptParallel(ctx context.Context, pod *v1.Pod, vol map[string]string, pgidDir string) (chan error, chan v1.ContainerStatus) {

	var wg sync.WaitGroup
	errChan := make(chan error, len(pod.Spec.Containers))
	cstatusChan := make(chan v1.ContainerStatus, len(pod.Spec.Containers))
	timeStart := metav1.NewTime(time.Now())

	for _, c := range pod.Spec.Containers {
		wg.Add(1)
		go func(c v1.Container) {
			defer wg.Done()
			log.G(ctx).WithField("container", c.Name).Info("Starting container")

			// get the scriptPath
			scriptMap, containerState, err := newCollectScripts(ctx, &c, pod.Name, vol)
			if err != nil {
				errChan <- err
				cstatusChan <- v1.ContainerStatus{
					Name:         c.Name,
					Image:        c.Image,
					ImageID: 	"",
					Ready:        false,
					RestartCount: 0,
					State:        *containerState,
				}
				return
			}

			scriptPath := scriptMap[c.Image]
			var command = c.Command
			if len(command) == 0 {
				log.G(ctx).WithField("container", c.Name).Errorf("No command found for container")
				errChan <- fmt.Errorf("no command found for container: %s", c.Name)
				return
			}else {
				// combine the command and scriptPath
				command = append(command, scriptPath)
			}

			var args string
			if len(c.Args) > 0 {
				args = strings.Join(c.Args, " ")
			}else{
				args = ""
			}

			env := c.Env
			args = strings.ReplaceAll(args, "~", os.Getenv("HOME"))
			args = strings.ReplaceAll(args, "$HOME", os.Getenv("HOME"))
			
			// find root of scriptPath for stdoutPath. Like /home/vscode/stress/job1/stress.sh -> /home/vscode/stress/job1
			stdoutPath := path.Dir(scriptPath)
			pgid, containerState, err := runScript(ctx, command, args, env, stdoutPath)

			if err != nil {
				errChan <- err
				cstatusChan <- v1.ContainerStatus{
					Name:         c.Name,
					Image:        c.Image,
					ImageID: 	scriptPath,
					Ready:        false,
					RestartCount: 0,
					State:        *containerState,
				}
				return
			}

			pgidFile := path.Join(pgidDir, fmt.Sprintf("%s_%s_%s.pgid", pod.Namespace, pod.Name, c.Name))
			log.G(ctx).WithField("pgid file path", pgidFile).Info("pgid file path")
			err = ioutil.WriteFile(pgidFile, []byte(fmt.Sprintf("%d", pgid)), 0644)
			if err != nil {
				containerState = &v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						Message:    fmt.Sprintf("failed to write pgid to file %s; error: %v", pgidFile, err),
						FinishedAt: metav1.NewTime(time.Now()),
						Reason:     "ContainerCreatingError",
						StartedAt:  timeStart,
					},
				}
				errChan <- err
				cstatusChan <- v1.ContainerStatus{
					Name:         c.Name,
					Image:        c.Image,
					ImageID: 	scriptPath,
					Ready:        false,
					RestartCount: 0,
					State: *containerState,
				}
				return
			}
		
			cstatusChan <- v1.ContainerStatus{
				Name:         c.Name,
				Image:        c.Image,
				ImageID: 	scriptPath,
				Ready:        false,
				RestartCount: 0,
				State: v1.ContainerState{
					Waiting: &v1.ContainerStateWaiting{
						Message:    fmt.Sprintf("container %s is waiting for the command to finish", c.Name),
						Reason:     "ContainerCreating",
					},
				},
			}
		}(c)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.G(ctx).WithField("error", r).Error("Recovered from panic while closing channels")
			}
		}()

		wg.Wait()

		close(errChan)
		log.G(ctx).Info("errChan closed")

		close(cstatusChan)
		log.G(ctx).Info("cstatusChan closed")
	}()

	return errChan, cstatusChan
}

func runScript(ctx context.Context, command []string, args string, env []v1.EnvVar, stdoutPath string) (int, *v1.ContainerState, error) {
	cmd := exec.Command("bash")

	// Create a map of environment variables
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		envMap[pair[0]] = pair[1]
	}

	// Update the environment variables with the provided ones
	for _, e := range env {
		e.Value = strings.ReplaceAll(e.Value, "~", os.Getenv("HOME"))
		e.Value = strings.ReplaceAll(e.Value, "$HOME", os.Getenv("HOME"))
		envMap[e.Name] = e.Value
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", e.Name, e.Value))
	}

	// Expand the command and arguments
	cmdString := strings.Join(command, " ")
	expand := func(s string) string {
		return envMap[s]
	}
	cmd.Args = append(cmd.Args, "-c", os.Expand(cmdString, expand) + " " + os.Expand(args, expand))
	log.G(ctx).WithField("command", cmd.Args).Info("command")
	// Set new process group id for the command 
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	
	// Open the stdout and stderr files
	stdoutFile, err := os.Create(path.Join(stdoutPath, "stdout"))
	if err != nil {
		log.G(ctx).WithField("command", cmdString).Errorf("failed to open stdout file; error: %v", err)
	}
	defer stdoutFile.Close()

	stderrFile, err := os.Create(path.Join(stdoutPath, "stderr"))
	if err != nil {
		log.G(ctx).WithField("command", cmdString).Errorf("failed to open stderr file; error: %v", err)
	}
	defer stderrFile.Close()

	// Set the stdout and stderr of the command
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile


	// Start the command
	err = cmd.Start()
	if err != nil {
		// Return a terminated container state with the exit code
		log.G(ctx).WithField("command", cmdString).Errorf("failed to start command; error: %v", err)
		return 0, &v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				ExitCode:   1,
				Reason:     "ContainerCreatingError",
				Message:    fmt.Sprintf("failed to start command; error: %v", err.Error()),
				FinishedAt: metav1.Now(),
			},
		}, err
	}


	// Get the process group id
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		log.G(ctx).WithField("command", cmdString).Errorf("failed to get process group id; error: %v", err)
		// Return a terminated container state with the exit code
		return 0, &v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				ExitCode:   1,
				Reason:     "ContainerCreatingError",
				Message:    fmt.Sprintf("failed to get process group id; error: %v", err),
				FinishedAt: metav1.Now(),
			},
		}, err
	}

	return pgid, nil, nil
}




func copyFile(ctx context.Context, src string, dst string, filename string) error {
	// create the destination directory if it does not exist
	err := exec.Command("mkdir", "-p", dst).Run()
	if err != nil {
		log.G(ctx).WithField("directory", dst).Errorf("failed to create directory; error: %v", err)
		return err
	}
	// mv the file to the destination directory
	err = exec.Command("cp", path.Join(src, filename), path.Join(dst, filename)).Run()
	if err != nil {
		log.G(ctx).WithFields(log.Fields{
			"source":      path.Join(src, filename),
			"destination": path.Join(dst, filename),
		}).Errorf("failed to copy file; error: %v", err)
		return err
	}
	return nil
}