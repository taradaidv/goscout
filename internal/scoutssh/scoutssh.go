// TODO:perform a major refactoring
package scoutssh

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"

	"github.com/kevinburke/ssh_config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func GetSSHHosts() ([]string, error) {
	sshConfigPath := filepath.Join(os.Getenv("HOME"), ".ssh", "config")

	file, err := os.Open(sshConfigPath)
	if err != nil {
		return nil, fmt.Errorf("opening SSH configuration: %w", err)
	}
	defer file.Close()

	configFile := bufio.NewReader(file)
	cfg, err := ssh_config.Decode(configFile)
	if err != nil {
		return nil, fmt.Errorf("parsing SSH configuration: %w", err)
	}

	var hosts []string
	for _, host := range cfg.Hosts {
		for _, pattern := range host.Patterns {
			hostName := pattern.String()
			if isSpecificHost(hostName) {
				hosts = append(hosts, hostName)
			}
		}
	}
	return hosts, nil
}

func isSpecificHost(host string) bool {
	for _, char := range host {
		if char == '*' || char == '?' {
			return false
		}
	}
	return true
}

func ConnectAndListFiles(host, path string) (*sftp.Client, *ssh.Client, map[string][]FileInfo, error) {
	configFile, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "config"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to open SSH configuration: %w", err)
	}
	defer configFile.Close()
	cfg, err := ssh_config.Decode(configFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse SSH configuration: %w", err)
	}

	hostname, _ := cfg.Get(host, "HostName")
	if hostname == "" {
		hostname = host
	}

	identityFile, _ := cfg.Get(host, "IdentityFile")
	if identityFile != "" {
		if identityFile[:2] == "~/" {
			identityFile = filepath.Join(os.Getenv("HOME"), identityFile[2:])
		}
		return connectWithIdentityFile(identityFile, host, hostname, path, cfg)
	}

	return connectWithoutIdentityFile(host, hostname, path, cfg)
}

func connectWithIdentityFile(identityFile, host, hostname, path string, cfg *ssh_config.Config) (*sftp.Client, *ssh.Client, map[string][]FileInfo, error) {
	port, _ := cfg.Get(host, "Port")
	if port == "" {
		port = "22"
	}

	user, _ := cfg.Get(host, "User")
	if user == "" {
		return nil, nil, nil, fmt.Errorf("user not found for host: %s", host)
	}

	key, err := os.ReadFile(identityFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read identity file: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", hostname+":"+port, sshConfig)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to connect to %s:%s - %w", hostname, port, err)
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	listings, err := FetchSFTPData(client, path)
	if err != nil {
		conn.Close()
		client.Close()
		return nil, nil, nil, err
	}

	return client, conn, listings, nil
}

func connectWithoutIdentityFile(host, hostname, path string, config *ssh_config.Config) (*sftp.Client, *ssh.Client, map[string][]FileInfo, error) {
	sshAgentSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAgentSock == "" {
		return nil, nil, nil, fmt.Errorf("SSH_AUTH_SOCK is not set")
	}

	sshAgent, err := net.Dial("unix", sshAgentSock)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to open SSH_AUTH_SOCK: %v", err)
	}

	agentClient := agent.NewClient(sshAgent)
	signers, err := agentClient.Signers()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get signers from agent: %v", err)
	}
	if len(signers) == 0 {
		return nil, nil, nil, fmt.Errorf("no signers found in SSH agent")
	}

	targetUser, _ := config.Get(host, "User")
	if targetUser == "" {
		currentUser, err := user.Current()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get current user: %v", err)
		}
		targetUser = currentUser.Username
	}

	proxyJump, _ := config.Get(host, "ProxyJump")
	if proxyJump == "" {
		return nil, nil, nil, fmt.Errorf("no ProxyJump configuration found for the target host")
	}

	proxyHost, _ := config.Get(proxyJump, "HostName")
	if proxyHost == "" {
		proxyHost = proxyJump
	}
	proxyUser, _ := config.Get(proxyJump, "User")
	if proxyUser == "" {
		currentUser, err := user.Current()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get current user: %v", err)
		}
		proxyUser = currentUser.Username
	}

	configSSH := &ssh.ClientConfig{
		User: proxyUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signers...),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	proxyClient, err := ssh.Dial("tcp", proxyHost+":22", configSSH)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to connect to proxy: %v", err)
	}

	targetConn, err := proxyClient.Dial("tcp", hostname+":22")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to connect to target through proxy: %v", err)
	}

	configSSH.User = targetUser
	ncc, chans, reqs, err := ssh.NewClientConn(targetConn, hostname+":22", configSSH)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create client connection: %v", err)
	}
	conn := ssh.NewClient(ncc, chans, reqs)

	client, err := sftp.NewClient(conn)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create SFTP client: %v", err)
	}

	listings, err := FetchSFTPData(client, path)
	if err != nil {
		sshAgent.Close()
		proxyClient.Close()
		targetConn.Close()
		conn.Close()
		client.Close()
		return nil, nil, nil, err
	}

	return client, conn, listings, nil
}

type FileInfo struct {
	Name string
	// Size    int64
	// ModTime time.Time
	IsDir  bool
	IsLink bool
	// Owner   string
	// Group   string
	// Perm string
}

func FetchSFTPData(client *sftp.Client, path string) (map[string][]FileInfo, error) {
	data := make(map[string][]FileInfo)
	entries, err := client.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		//TODO:boost
		//fullPath := filepath.Join(path, entry.Name())
		// stat, err := client.Lstat(fullPath)
		// if err != nil {
		// 	return nil, err
		// }

		// fileStat, ok := stat.Sys().(*sftp.FileStat)
		// if !ok {
		// 	return nil, fmt.Errorf("failed to assert file stat")
		// }

		// owner := fmt.Sprintf("%d", fileStat.UID)
		// group := fmt.Sprintf("%d", fileStat.GID)

		fileInfo := FileInfo{
			Name: entry.Name(),
			// Size:    entry.Size(),
			// ModTime: entry.ModTime(),
			IsDir:  entry.IsDir(),
			IsLink: entry.Mode()&os.ModeSymlink != 0,
			// Owner:   owner,
			// Group:   group,
			// Perm: entry.Mode().Perm().String(),
		}

		dir := path
		data[dir] = append(data[dir], fileInfo)
	}

	return data, nil
}
