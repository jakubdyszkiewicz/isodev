package container

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/jakubdyszkiewicz/isodev/internal/config"
	"github.com/jakubdyszkiewicz/isodev/internal/shell"
)

// paseoEnv identifies the Paseo agent to commands launched for Paseo.
var paseoEnv = []string{"PASEO_AGENT_ID", "PASEO_AGENT_CWD"}

// forwardedEnvArgs passes names rather than values to Docker so secrets are not
// exposed in process arguments. Docker reads their values from this process.
func forwardedEnvArgs(names []string, paseo bool) []string {
	if paseo {
		names = append(append([]string{}, names...), paseoEnv...)
	}
	seen := map[string]bool{}
	var args []string
	for _, name := range names {
		if _, present := os.LookupEnv(name); present && !seen[name] {
			args = append(args, "--env", name)
			seen[name] = true
		}
	}
	return args
}

// launchScript records the command's process group so it can be terminated on
// interrupt, then replaces itself with the command.
const launchScript = `echo $$ > "$1"; shift; exec "$@"`

func execArgs(container string, args []string, forwardPaseoEnv bool, pidFile string) ([]string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	a := []string{"exec", "-i"}
	a = append(a, forwardedEnvArgs(cfg.Lifecycle.OnExec.ForwardEnvVars, forwardPaseoEnv)...)
	a = append(a, container, "setsid", "--wait", "sh", "-c", launchScript, "isodev-launch", pidFile)
	return append(a, args...), nil
}

// Launch runs a command in the container attached to this process's stdio.
// forwardPaseoEnv also forwards the Paseo agent identity variables.
func Launch(container string, args []string, forwardPaseoEnv bool) error {
	return LaunchIO(container, args, forwardPaseoEnv, os.Stdin, os.Stdout)
}

// LaunchIO is Launch with explicit standard input and output. On SIGINT or
// SIGTERM, the command's whole process group in the container is terminated.
func LaunchIO(container string, args []string, forwardPaseoEnv bool, input io.Reader, output io.Writer) error {
	if container == "" || len(args) == 0 || args[0] == "" {
		return fmt.Errorf("exec requires a container and command")
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	pid := "/tmp/isodev-" + hex.EncodeToString(token[:]) + ".pid"
	a, err := execArgs(container, args, forwardPaseoEnv, pid)
	if err != nil {
		return err
	}
	c := exec.Command("docker", a...)
	c.Stdin = input
	c.Stdout = output
	c.Stderr = os.Stderr
	if err = c.Start(); err != nil {
		return err
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	stopGroup := func() {
		_, _ = shell.Run("docker", "exec", container, "sh", "-c", `if test -f "$1"; then p=$(cat "$1"); /bin/kill -TERM -- "-$p" 2>/dev/null || true; fi`, "cleanup", pid)
	}
	select {
	case err = <-done:
		// The terminal sends Ctrl-C to the Docker CLI too, which can exit
		// before Isodev sees the signal; stop the commands it leaves behind.
		select {
		case <-ch:
			stopGroup()
		default:
		}
	case sig := <-ch:
		stopGroup()
		_ = c.Process.Signal(sig)
		if input != os.Stdin {
			if closer, ok := input.(io.Closer); ok {
				_ = closer.Close()
			}
		}
		err = <-done
	}
	signal.Stop(ch)
	_, _ = shell.Run("docker", "exec", container, "rm", "-f", pid)
	return err
}

func enterArgs(container string) []string {
	return []string{"exec", "-it", container, "sh", "-c", `if command -v fish >/dev/null 2>&1; then exec fish -i; elif command -v bash >/dev/null 2>&1; then exec bash -i; else exec sh -i; fi`}
}

// Enter opens an interactive shell in the container: Fish if installed,
// otherwise Bash or sh.
func Enter(r Record) error {
	c := exec.Command("docker", enterArgs(r.Container)...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}
