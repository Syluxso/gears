package cmd

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	qrterminal "github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"
)

var ionicServeFlag bool
var ionicPublicFlag bool
var ionicPort int
var ionicTunnel bool

var ionicCmd = &cobra.Command{
	Use:   "ionic",
	Short: "Ionic development utilities",
	Long: `Utilities for Ionic app development.

Examples:
  gears ionic -s                     Serve locally on http://localhost:8100
  gears ionic -s -p                  Serve publicly on local network (shows all detected Network URLs + QR for best reachable one)
  gears ionic -s -p --tunnel         Tunnel both DDEV backend (for API) and Ionic frontend via Cloudflare.
                                     QR code will use the public tunnel URL. The dev server is started with
                                     --disable-host-check so the tunnel Host header is accepted.
  gears ionic -s -p --port 3000      Use a custom port`,
	RunE: runIonic,
}

func init() {
	rootCmd.AddCommand(ionicCmd)
	ionicCmd.Flags().BoolVarP(&ionicServeFlag, "serve", "s", false, "Start ionic serve")
	ionicCmd.Flags().BoolVarP(&ionicPublicFlag, "public", "p", false, "Serve publicly on local network and show QR code")
	ionicCmd.Flags().IntVar(&ionicPort, "port", 8100, "Port to serve on")
	ionicCmd.Flags().BoolVar(&ionicTunnel, "tunnel", false, "Start a Cloudflare tunnel for the DDEV backend and auto-patch environment.ts (reverts on exit)")
}

func runIonic(cmd *cobra.Command, args []string) error {
	if !ionicServeFlag {
		return cmd.Help()
	}

	port := strconv.Itoa(ionicPort)

	if ionicTunnel {
		ddevPort, err := ionicFindDDEVPort()
		if err != nil {
			return fmt.Errorf("could not find DDEV project: %w\nTip: run from your Ionic project directory alongside a DDEV project", err)
		}

		fmt.Printf("\n  Found DDEV on port %s\n", ddevPort)
		fmt.Println("  Starting Cloudflare tunnel...")

		tunnelURL, tunnelProc, err := ionicStartCloudflareTunnel(ddevPort)
		if err != nil {
			return err
		}

		fmt.Printf("  Tunnel:  %s\n", tunnelURL)

		envFile, originalURL, err := ionicPatchEnvFile(tunnelURL)
		if err != nil {
			_ = tunnelProc.Kill()
			return err
		}

		fmt.Printf("  Patched: %s\n", envFile)

		var cleanupOnce sync.Once
		cleanup := func() {
			cleanupOnce.Do(func() {
				fmt.Println("\n  Reverting environment.ts...")
				_ = ionicRevertEnvFile(envFile, originalURL)
				fmt.Println("  Stopping Cloudflare tunnel...")
				_ = tunnelProc.Kill()
			})
		}
		defer cleanup()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			cleanup()
			os.Exit(0)
		}()
	}

	if ionicPublicFlag {
		ips, _ := ionicGetLocalIPs() // ignore err, we'll handle

		fmt.Println()
		for _, ip := range ips {
			fmt.Printf("  Network URL: http://%s:%s\n", ip, port)
		}

		var qrURL string
		var frontendTunnelProc *os.Process

		if ionicTunnel {
			fmt.Println("  Starting Cloudflare tunnel for the Ionic frontend (so phone can reach the dev server via public URL)...")
			var err error
			qrURL, frontendTunnelProc, err = ionicStartCloudflareTunnel(port)
			if err != nil {
				fmt.Printf("  Warning: frontend tunnel failed (%v). Falling back to local IP.\n", err)
				if len(ips) > 0 {
					qrURL = fmt.Sprintf("http://%s:%s", ips[0], port)
				}
			} else {
				fmt.Printf("  Frontend Public URL (via tunnel for phone): %s\n", qrURL)
			}
		} else if len(ips) > 0 {
			// Prefer non-172.x (WSL/Docker internal networks phones on LAN usually can't reach directly)
			for _, ip := range ips {
				if !strings.HasPrefix(ip, "172.") {
					qrURL = fmt.Sprintf("http://%s:%s", ip, port)
					break
				}
			}
			if qrURL == "" {
				qrURL = fmt.Sprintf("http://%s:%s", ips[0], port)
			}
		}

		if qrURL != "" {
			fmt.Println()
			qrterminal.GenerateHalfBlock(qrURL, qrterminal.L, os.Stdout)
			fmt.Println()
		}

		// If we started a frontend tunnel, make sure we clean it up on exit
		if frontendTunnelProc != nil {
			// extend existing cleanup if present, or create new
			// for simplicity, add a separate defer here
			defer func() {
				fmt.Println("\n  Stopping frontend Cloudflare tunnel...")
				_ = frontendTunnelProc.Kill()
			}()
		}
	}

	return ionicStartServe(port, ionicPublicFlag)
}

func ionicFindDDEVPort() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	ddevDir, err := ionicFindDDEVDir(cwd)
	if err != nil {
		return "", err
	}

	c := exec.Command("ddev", "describe")
	c.Dir = ddevDir
	out, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("ddev describe failed in %s: %w", ddevDir, err)
	}

	re := regexp.MustCompile(`web:80 -> 127\.0\.0\.1:(\d+)`)
	m := re.FindSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("could not parse HTTP port from ddev describe output")
	}

	return string(m[1]), nil
}

// isDDEVProject checks for config.yaml to distinguish a real DDEV project
// from the global ~/.ddev config directory.
func isDDEVProject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".ddev", "config.yaml"))
	return err == nil
}

func ionicFindDDEVDir(start string) (string, error) {
	// Walk up from start looking for a real .ddev project
	dir := start
	for {
		if isDDEVProject(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Also check siblings at each level (up to 3 levels up)
	dir = start
	for i := 0; i < 3; i++ {
		parent := filepath.Dir(dir)
		entries, err := os.ReadDir(parent)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				candidate := filepath.Join(parent, e.Name())
				if isDDEVProject(candidate) {
					return candidate, nil
				}
			}
		}
		dir = parent
	}

	return "", fmt.Errorf("no DDEV project found near %s", start)
}

func ionicStartCloudflareTunnel(port string) (string, *os.Process, error) {
	c := exec.Command("cloudflared", "tunnel", "--url", "http://127.0.0.1:"+port)

	stderr, err := c.StderrPipe()
	if err != nil {
		return "", nil, fmt.Errorf("could not pipe cloudflared stderr: %w", err)
	}

	if err := c.Start(); err != nil {
		return "", nil, fmt.Errorf("could not start cloudflared: %w\nIs it installed? Run: winget install Cloudflare.cloudflared", err)
	}

	urlCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stderr)
		re := regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)
		for scanner.Scan() {
			if m := re.FindString(scanner.Text()); m != "" {
				urlCh <- m
				// keep draining so the process doesn't block on a full pipe
				for scanner.Scan() {
				}
				return
			}
		}
		close(urlCh)
	}()

	select {
	case url, ok := <-urlCh:
		if !ok {
			_ = c.Process.Kill()
			return "", nil, fmt.Errorf("cloudflared exited without providing a tunnel URL")
		}
		return url, c.Process, nil
	case <-time.After(30 * time.Second):
		_ = c.Process.Kill()
		return "", nil, fmt.Errorf("timed out waiting for Cloudflare tunnel URL")
	}
}

func ionicPatchEnvFile(tunnelURL string) (path string, originalURL string, err error) {
	envFile, err := ionicFindEnvFile()
	if err != nil {
		return "", "", err
	}

	data, err := os.ReadFile(envFile)
	if err != nil {
		return "", "", fmt.Errorf("could not read %s: %w", envFile, err)
	}

	re := regexp.MustCompile(`apiUrl:\s*['"]([^'"]+)['"]`)
	m := re.FindSubmatch(data)
	if m == nil {
		return "", "", fmt.Errorf("could not find apiUrl in %s", envFile)
	}
	originalURL = string(m[1])

	newURL := tunnelURL + ionicExtractPath(originalURL)
	patched := re.ReplaceAll(data, []byte("apiUrl: '"+newURL+"'"))

	if err := os.WriteFile(envFile, patched, 0644); err != nil {
		return "", "", fmt.Errorf("could not write %s: %w", envFile, err)
	}

	return envFile, originalURL, nil
}

func ionicRevertEnvFile(path, originalURL string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`apiUrl:\s*['"]([^'"]+)['"]`)
	reverted := re.ReplaceAll(data, []byte("apiUrl: '"+originalURL+"'"))
	return os.WriteFile(path, reverted, 0644)
}

func ionicFindEnvFile() (string, error) {
	candidates := []string{
		filepath.Join("src", "environments", "environment.ts"),
		filepath.Join("environments", "environment.ts"),
	}

	// Check from CWD
	for _, rel := range candidates {
		if _, err := os.Stat(rel); err == nil {
			return rel, nil
		}
	}

	// Walk up 2 levels
	cwd, _ := os.Getwd()
	for i := 0; i < 2; i++ {
		cwd = filepath.Dir(cwd)
		for _, rel := range candidates {
			p := filepath.Join(cwd, rel)
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	return "", fmt.Errorf("could not find src/environments/environment.ts — run from your Ionic project root")
}

func ionicExtractPath(rawURL string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(rawURL, prefix) {
			rest := rawURL[len(prefix):]
			if idx := strings.Index(rest, "/"); idx >= 0 {
				return rest[idx:]
			}
			return ""
		}
	}
	return ""
}

func ionicGetLocalIPs() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var ips []string
	seen := map[string]bool{}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			if ip[0] == 169 && ip[1] == 254 {
				continue
			}
			s := ip.String()
			if seen[s] {
				continue
			}
			seen[s] = true
			ips = append(ips, s)
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no valid local IP address found — check your network connection")
	}
	return ips, nil
}

func ionicStartServe(port string, public bool) error {
	serveArgs := []string{"serve", "--port", port}
	if public {
		serveArgs = append(serveArgs, "--external", "--host", "0.0.0.0")
	}

	// When using Cloudflare tunnel for the frontend (via --tunnel + -p),
	// the requests come with a Host header like *.trycloudflare.com.
	// Ionic's underlying dev server (webpack-dev-server / ng serve) rejects
	// "invalid host header" by default for security.
	// We disable the check when tunneling so the phone can load the dev server
	// over the public tunnel URL that appears in the QR code.
	if ionicTunnel && public {
		serveArgs = append(serveArgs, "--", "--disable-host-check")
	}

	fmt.Printf("  Running: ionic %s\n\n", strings.Join(serveArgs, " "))
	c := exec.Command("ionic", serveArgs...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
