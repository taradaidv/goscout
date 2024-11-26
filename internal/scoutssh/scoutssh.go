package scoutssh

import (
	"bufio"
	"errors"
	"fmt"
	"sync"

	"net"
	"os"
	"os/user"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/kevinburke/ssh_config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const path = "."

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

func Connect(w fyne.Window, host, secret string) (*sftp.Client, *ssh.Client, map[string][]FileInfo, error) {
	configFile, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "config"))
	if err != nil {
		return nil, nil, nil, err
	}
	defer configFile.Close()
	cfg, err := ssh_config.Decode(configFile)
	if err != nil {
		return nil, nil, nil, err
	}

	hostname, err := cfg.Get(host, "HostName")
	if err != nil {
		return nil, nil, nil, err
	}
	if hostname == "" {
		hostname = host
	}

	var authMethods []ssh.AuthMethod
	identity, _ := cfg.Get(host, "IdentityFile")
	if identity != "" {
		if identity[:2] == "~/" {
			identity = filepath.Join(os.Getenv("HOME"), identity[2:])
		}
		key, err := os.ReadFile(identity)
		if err != nil {
			return nil, nil, nil, err
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, nil, nil, err
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		sshAgentSock := os.Getenv("SSH_AUTH_SOCK")
		if sshAgentSock == "" {
			return nil, nil, nil, fmt.Errorf("SSH_AUTH_SOCK is not set")
		}

		sshAgent, err := net.Dial("unix", sshAgentSock)
		if err != nil {
			return nil, nil, nil, err
		}
		defer sshAgent.Close()

		agentClient := agent.NewClient(sshAgent)
		signers, err := agentClient.Signers()
		if err != nil {
			return nil, nil, nil, err
		}
		if len(signers) == 0 {
			return nil, nil, nil, fmt.Errorf("no signers found in SSH agent")
		}
		authMethods = append(authMethods, ssh.PublicKeys(signers...))
	}

	username, err := cfg.Get(host, "User")
	if err != nil {
		return nil, nil, nil, err
	}
	if username == "" {
		currentUser, err := user.Current()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get current user: %v", err)
		}
		username = currentUser.Username
	}

	port, err := cfg.Get(host, "Port")
	if err != nil {
		return nil, nil, nil, err
	}
	if port == "" {
		port = "22"
	}

	proxyJump, _ := cfg.Get(host, "ProxyJump")
	if proxyJump != "" {
		proxyHost, _ := cfg.Get(proxyJump, "HostName")
		if proxyHost == "" {
			proxyHost = proxyJump
		}
		proxyUser, _ := cfg.Get(proxyJump, "User")
		if proxyUser == "" {
			currentUser, err := user.Current()
			if err != nil {
				return nil, nil, nil, err
			}
			proxyUser = currentUser.Username
		}

		proxyConfig := &ssh.ClientConfig{
			User:            proxyUser,
			Auth:            authMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		proxyClient, err := ssh.Dial("tcp", proxyHost+":22", proxyConfig)
		if err != nil {
			return nil, nil, nil, err
		}

		targetConn, err := proxyClient.Dial("tcp", hostname+":"+port)
		if err != nil {
			return nil, nil, nil, err
		}

		sshConfig := &ssh.ClientConfig{
			User:            username,
			Auth:            authMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		ncc, chans, reqs, err := ssh.NewClientConn(targetConn, hostname+":"+port, sshConfig)
		if err != nil {
			password := RequestPassword(host, hostname, secret, w)
			if password == "" {
				return nil, nil, nil, errors.New("password auth decline")
			}
			authMethods = append(authMethods, ssh.Password(password))
			sshConfig.Auth = authMethods

			ncc, chans, reqs, err = ssh.NewClientConn(targetConn, hostname+":"+port, sshConfig)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		sshClient := ssh.NewClient(ncc, chans, reqs)

		sftpClient, err := sftp.NewClient(sshClient)
		if err != nil {
			sshClient.Close()
			return nil, nil, nil, err
		}

		listings, err := FetchSFTPData(sftpClient, path)
		if err != nil {
			sshClient.Close()
			sftpClient.Close()
			return nil, nil, nil, err
		}

		return sftpClient, sshClient, listings, nil
	}

	sshConfig := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshClient, err := ssh.Dial("tcp", hostname+":"+port, sshConfig)

	if err != nil {
		password := RequestPassword(host, hostname, secret, w)
		if password == "" {
			return nil, nil, nil, errors.New("password auth decline")
		}
		authMethods = append(authMethods, ssh.Password(password))
		sshConfig.Auth = authMethods

		sshClient, err = ssh.Dial("tcp", hostname+":"+port, sshConfig)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, nil, nil, err
	}

	listings, err := FetchSFTPData(sftpClient, path)
	if err != nil {
		sshClient.Close()
		sftpClient.Close()
		return nil, nil, nil, err
	}

	return sftpClient, sshClient, listings, nil
}

func RequestPassword(host, hostname, secret string, w fyne.Window) string {

	passwordChan := make(chan string)
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetText(secret)
	dialog.ShowCustomConfirm(host+" / "+hostname, "OK", "Cancel",
		container.NewVBox(widget.NewLabel("ssh password auth"), passwordEntry),
		func(ok bool) {
			if ok {
				passwordChan <- passwordEntry.Text
			} else {
				passwordChan <- ""
			}
		}, w)

	return <-passwordChan
}

type FileInfo struct {
	Name     string
	IsDir    bool
	IsLink   bool
	FullPath string
}

func FetchSFTPData(client *sftp.Client, path string) (map[string][]FileInfo, error) {
	data := make(map[string][]FileInfo)
	entries, err := client.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	fileInfoChan := make(chan FileInfo, len(entries))
	workerCount := 10

	worker := func(entries <-chan os.FileInfo) {
		for entry := range entries {
			fullPath := filepath.Join(path, entry.Name())
			isLink := entry.Mode()&os.ModeSymlink != 0
			isDir := entry.IsDir()
			name := entry.Name()

			if isLink {
				name += "*"
				if realPath, err := client.ReadLink(fullPath); err == nil {
					if linkInfo, err := client.Stat(fullPath); err == nil && linkInfo.IsDir() {
						fullPath = "/" + realPath + "/"
						isDir = true
					}
				}
			} else if isDir {
				fullPath += "/"
			}

			fileInfo := FileInfo{
				Name:     name,
				FullPath: fullPath,
				IsDir:    isDir,
				IsLink:   isLink,
			}

			fileInfoChan <- fileInfo
		}
		wg.Done()
	}

	entryChan := make(chan os.FileInfo, len(entries))
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker(entryChan)
	}

	go func() {
		for _, entry := range entries {
			entryChan <- entry
		}
		close(entryChan)
	}()

	go func() {
		wg.Wait()
		close(fileInfoChan)
	}()

	for fileInfo := range fileInfoChan {
		mu.Lock()
		data[path] = append(data[path], fileInfo)
		mu.Unlock()
	}

	return data, nil
}
