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

func newCollectScripts(ctx context.Context, container *v1.Container, podName string, volumeMap map[string]string) (map[string]string, *v1.ContainerState, error) {
	startTime := metav1.NewTime(time.Now())

	// Define a map to store the bash scripts, with the container name as the key and the list of bash scripts as the value
	scriptMap := make(map[string]string)
	var containerState *v1.ContainerState

	// Iterate over each volume mount in the container
	for _, volumeMount := range container.VolumeMounts {
		defaultVolumeDirectory := volumeMap[volumeMount.Name]
		mountDirectory := path.Join(os.Getenv("HOME"), podName, "containers", volumeMount.MountPath)

		log.G(ctx).WithField("volume name", volumeMount.Name).WithField("mount directory", mountDirectory).Info("Processing volumeMount")

		// Scan the default volume directory for files
		files, err := ioutil.ReadDir(defaultVolumeDirectory)
		if err != nil {
			log.G(ctx).WithField("default volume directory", defaultVolumeDirectory).Errorf("Failed to read default volume directory; error: %v", err)
			
			containerState = &v1.ContainerState{
				Terminated: &v1.ContainerStateTerminated{
					Message:    fmt.Sprintf("Failed to read default volume directory %s; error: %v", defaultVolumeDirectory, err),
					FinishedAt: metav1.NewTime(time.Now()),
					Reason:     "ContainerCreatingError",
					StartedAt:  startTime,
				},
			}
			return nil, containerState, err
		}

		// Iterate over each file in the default volume directory
		for _, file := range files {
			log.G(ctx).WithField("File name", file.Name()).Info("File in default volume directory")

			// If the file name contains "crt", "key", or "pem", skip it
			if strings.Contains(file.Name(), "crt") || strings.Contains(file.Name(), "key") || strings.Contains(file.Name(), "pem") {
				log.G(ctx).WithField("file_name", file.Name()).Info("File name contains crt, key, or pem, skipping it")
				continue
			}

			// Copy the file to the mount directory
			err := copyFile(ctx, defaultVolumeDirectory, mountDirectory, file.Name())
			if err != nil {
				log.G(ctx).WithField("File name", file.Name()).Errorf("Failed to copy file; error: %v", err)

				containerState = &v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						Message:    fmt.Sprintf("Failed to copy file %s to %s; error: %v", path.Join(defaultVolumeDirectory, file.Name()), path.Join(mountDirectory, file.Name()), err),
						FinishedAt: metav1.NewTime(time.Now()),
						Reason:     "ContainerCreatingError",
						StartedAt:  startTime,
					},
				}
				return nil, containerState, err
			}

			// Add the script path to the script map
			scriptPath := path.Join(mountDirectory, file.Name())
			scriptMap[volumeMount.Name] = scriptPath
		}
	}

	return scriptMap, nil, nil
}

func (p *MockProvider) runScriptParallel(ctx context.Context, pod *v1.Pod, volumeMap map[string]string, pgidDir string) (chan error, chan v1.ContainerStatus) {
	var wg sync.WaitGroup
	errorChannel := make(chan error, len(pod.Spec.Containers))
	containerStatusChannel := make(chan v1.ContainerStatus, len(pod.Spec.Containers))
	startTime := metav1.NewTime(time.Now())

	for _, container := range pod.Spec.Containers {
		wg.Add(1)
		go func(container v1.Container) {
			defer wg.Done()
			log.G(ctx).WithField("container", container.Name).Info("Starting container")

			// Collect scripts for the container
			scriptMap, containerState, err := newCollectScripts(ctx, &container, pod.Name, volumeMap)
			if err != nil {
				errorChannel <- err
				containerStatusChannel <- generateContainerStatus(container, "", false, containerState)
				return
			}

			// Get the script path for the container image
			scriptPath := scriptMap[container.Image]
			command := container.Command
			if len(command) == 0 {
				log.G(ctx).WithField("container", container.Name).Errorf("No command found for container")
				errorChannel <- fmt.Errorf("no command found for container: %s", container.Name)
				return
			}

			// Combine the command and scriptPath
			command = append(command, scriptPath)

			// Prepare the arguments for the command
			args := prepareArgs(container.Args)

			// Run the script and get the process group ID
			pgid, containerState, err := runScript(ctx, command, args, container.Env, path.Dir(scriptPath))
			if err != nil {
				errorChannel <- err
				containerStatusChannel <- generateContainerStatus(container, scriptPath, false, containerState)
				return
			}

			// Write the process group ID to a file
			pgidFile := path.Join(pgidDir, fmt.Sprintf("%s_%s_%s.pgid", pod.Namespace, pod.Name, container.Name))
			log.G(ctx).WithField("pgid file path", pgidFile).Info("pgid file path")
			err = ioutil.WriteFile(pgidFile, []byte(fmt.Sprintf("%d", pgid)), 0644)
			if err != nil {
				containerState = &v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						Message:    fmt.Sprintf("failed to write pgid to file %s; error: %v", pgidFile, err),
						FinishedAt: metav1.NewTime(time.Now()),
						Reason:     "ContainerCreatingError",
						StartedAt:  startTime,
					},
				}
				errorChannel <- err
				containerStatusChannel <- generateContainerStatus(container, scriptPath, false, containerState)
				return
			}

			// Send the container status to the channel
			containerStatusChannel <- generateContainerStatus(container, scriptPath, false, &v1.ContainerState{
				Waiting: &v1.ContainerStateWaiting{
					Message: fmt.Sprintf("container %s is waiting for the command to finish", container.Name),
					Reason:  "ContainerCreating",
				},
			})
		}(container)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.G(ctx).WithField("error", r).Error("Recovered from panic while closing channels")
			}
		}()

		wg.Wait()

		close(errorChannel)
		log.G(ctx).Info("errorChannel closed")

		close(containerStatusChannel)
		log.G(ctx).Info("containerStatusChannel closed")
	}()

	return errorChannel, containerStatusChannel
}

func generateContainerStatus(container v1.Container, scriptPath string, ready bool, state *v1.ContainerState) v1.ContainerStatus {
	return v1.ContainerStatus{
		Name:         container.Name,
		Image:        container.Image,
		ImageID:      scriptPath,
		Ready:        ready,
		RestartCount: 0,
		State:        *state,
	}
}

func prepareArgs(args []string) string {
	if len(args) > 0 {
		return strings.Join(args, " ")
	}
	return ""
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