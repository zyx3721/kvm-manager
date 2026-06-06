package kvm

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const migrationConnectionTimeout = 10 * time.Second

func (p *VirshProvider) CheckMigrationConnection(request MigrationConnectionCheckRequest) (MigrationConnectionCheckResult, error) {
	destinationURI := strings.TrimSpace(request.DestinationURI)
	if destinationURI == "" {
		return MigrationConnectionCheckResult{}, fmt.Errorf("destination uri is required")
	}
	if _, err := url.ParseRequestURI(destinationURI); err != nil {
		return MigrationConnectionCheckResult{}, fmt.Errorf("destination uri is invalid")
	}
	if !isQemuSSHMigrationURI(destinationURI) {
		return MigrationConnectionCheckResult{OK: true, Message: "当前迁移 URI 不是 qemu+ssh，跳过 SSH 免密检测"}, nil
	}
	checkURI := migrationURIWithNonInteractiveFlags(destinationURI)
	if _, err := p.outputWithTimeout(migrationConnectionTimeout, "virsh", "--connect", checkURI, "list", "--all"); err != nil {
		message := strings.Join(strings.Fields(err.Error()), " ")
		return MigrationConnectionCheckResult{
			OK:               false,
			PasswordRequired: migrationSSHPasswordRequired(message),
			Message:          migrationConnectionMessage(message),
		}, nil
	}
	if request.Live {
		target, err := migrationSSHTarget(destinationURI, "")
		if err != nil {
			return MigrationConnectionCheckResult{}, err
		}
		if err := p.checkMigrationTargetHostname(target, checkURI); err != nil {
			return MigrationConnectionCheckResult{
				OK:      false,
				Message: migrationConnectionMessage(err.Error()),
			}, nil
		}
	}
	return MigrationConnectionCheckResult{OK: true, Message: "迁移通道可用"}, nil
}

func (p *VirshProvider) SetupMigrationSSHKey(request MigrationSSHKeySetupRequest) (MigrationConnectionCheckResult, error) {
	target, err := migrationSSHTarget(request.DestinationURI, request.Username)
	if err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	if strings.TrimSpace(request.Password) == "" {
		return MigrationConnectionCheckResult{}, fmt.Errorf("ssh password is required")
	}
	publicKey, err := p.ensureMigrationPublicKey()
	if err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	if err := installMigrationPublicKey(target, request.Password, publicKey); err != nil {
		return MigrationConnectionCheckResult{
			OK:               false,
			PasswordRequired: true,
			Message:          "目标宿主机 SSH 认证失败，请确认用户名和密码正确",
		}, nil
	}
	_ = p.rememberMigrationHostKey(target)
	return p.CheckMigrationConnection(MigrationConnectionCheckRequest{DestinationURI: request.DestinationURI})
}

func (p *VirshProvider) SetupMigrationHostname(request MigrationHostnameSetupRequest) (MigrationConnectionCheckResult, error) {
	target, err := migrationSSHTarget(request.DestinationURI, "")
	if err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	hostname, err := normalizeMigrationHostname(request.Hostname)
	if err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	targetIP := target.Host
	if net.ParseIP(targetIP) == nil {
		ips, err := net.LookupIP(target.Host)
		if err != nil || len(ips) == 0 {
			return MigrationConnectionCheckResult{}, fmt.Errorf("migration target host ip is unavailable")
		}
		targetIP = ips[0].String()
	}
	if err := p.applyMigrationTargetHostname(target, targetIP, hostname); err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	if err := p.ensureLocalHostsEntry(targetIP, hostname); err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	return p.CheckMigrationConnection(MigrationConnectionCheckRequest{DestinationURI: request.DestinationURI, Live: true})
}

func migrationURIWithNonInteractiveFlags(destinationURI string) string {
	parsed, err := url.Parse(destinationURI)
	if err != nil {
		return destinationURI
	}
	if !isQemuSSHMigrationURI(destinationURI) {
		return destinationURI
	}
	query := parsed.Query()
	if query.Get("no_tty") == "" {
		query.Set("no_tty", "1")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func isQemuSSHMigrationURI(destinationURI string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(destinationURI)), "qemu+ssh://")
}

func migrationSSHPasswordRequired(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "permission denied") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "ssh_askpass") ||
		strings.Contains(lower, "no tty")
}

func migrationConnectionMessage(message string) string {
	if migrationSSHPasswordRequired(message) {
		return "源宿主机无法免密连接目标 libvirt，请先配置 SSH 免密"
	}
	if migrationTargetHostnameLocalhost(message) {
		return "目标宿主机主机名解析为 localhost，热迁移需要目标主机名解析到真实网络地址，请检查目标宿主机 hostname"
	}
	if strings.Contains(strings.ToLower(message), "host key verification failed") {
		return "源宿主机尚未信任目标宿主机 SSH 指纹，请先配置迁移 SSH 免密"
	}
	if message == "" {
		return "迁移通道不可用"
	}
	return message
}

func normalizeMigrationHostname(hostname string) (string, error) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if migrationHostnameLooksLocalhost(hostname) {
		return "", fmt.Errorf("migration target hostname must not be localhost")
	}
	if strings.ContainsAny(hostname, " \t\r\n/\\:") {
		return "", fmt.Errorf("migration target hostname is invalid")
	}
	if hostname == "" || len(hostname) > 253 {
		return "", fmt.Errorf("migration target hostname is invalid")
	}
	for _, label := range strings.Split(hostname, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", fmt.Errorf("migration target hostname is invalid")
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return "", fmt.Errorf("migration target hostname is invalid")
			}
		}
	}
	return hostname, nil
}

func (p *VirshProvider) applyMigrationTargetHostname(target migrationTarget, targetIP string, hostname string) error {
	if _, err := p.outputWithTimeout(migrationConnectionTimeout, "ssh", append(migrationSSHArgs(target), "hostnamectl", "set-hostname", hostname)...); err != nil {
		return err
	}
	command := migrationHostsEntryCommand(targetIP, hostname)
	if _, err := p.outputWithTimeout(migrationConnectionTimeout, "ssh", migrationRemoteShellArgs(target, command)...); err != nil {
		return err
	}
	return nil
}

func (p *VirshProvider) ensureLocalHostsEntry(ip string, hostname string) error {
	_, err := p.outputWithTimeout(migrationConnectionTimeout, "sh", "-c", migrationHostsEntryCommand(ip, hostname))
	return err
}

func migrationHostsEntryCommand(ip string, hostname string) string {
	entry := shellQuote(strings.TrimSpace(ip) + " " + strings.TrimSpace(hostname))
	script := shellQuote("/(^|[[:space:]])" + strings.TrimSpace(hostname) + "([[:space:]]|$)/d")
	return "sed -i.bak -E " + script + " /etc/hosts && printf '%s\\n' " + entry + " >> /etc/hosts"
}

func (p *VirshProvider) checkMigrationTargetHostname(target migrationTarget, checkURI string) error {
	libvirtHostname, err := p.outputWithTimeout(migrationConnectionTimeout, "virsh", "--connect", checkURI, "hostname")
	if err != nil {
		return err
	}
	if migrationHostnameLooksLocalhost(libvirtHostname) {
		return fmt.Errorf("hostname on destination resolved to localhost, but migration requires an FQDN")
	}
	if err := p.checkRemoteHostnameResolution(target, strings.TrimSpace(libvirtHostname)); err != nil {
		return err
	}
	hostname, err := p.outputWithTimeout(migrationConnectionTimeout, "ssh", append(migrationSSHArgs(target), "hostname")...)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(hostname)
	if migrationHostnameLooksLocalhost(name) {
		return fmt.Errorf("hostname on destination resolved to localhost, but migration requires an FQDN")
	}
	if err := p.checkRemoteHostnameResolution(target, name); err != nil {
		return err
	}
	fqdn, err := p.outputWithTimeout(migrationConnectionTimeout, "ssh", append(migrationSSHArgs(target), "hostname", "-f")...)
	if err != nil {
		return err
	}
	fqdn = strings.TrimSpace(fqdn)
	if migrationHostnameLooksLocalhost(fqdn) {
		return fmt.Errorf("hostname on destination resolved to localhost, but migration requires an FQDN")
	}
	if fqdn != "" && fqdn != name {
		return p.checkRemoteHostnameResolution(target, fqdn)
	}
	return nil
}

func (p *VirshProvider) checkRemoteHostnameResolution(target migrationTarget, hostname string) error {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return fmt.Errorf("hostname on destination resolved to localhost, but migration requires an FQDN")
	}
	command := "getent hosts " + shellQuote(hostname) + " || true"
	resolved, err := p.outputWithTimeout(migrationConnectionTimeout, "ssh", migrationRemoteShellArgs(target, command)...)
	if err != nil {
		return err
	}
	if strings.TrimSpace(resolved) == "" || migrationTargetHostnameLocalhost(resolved) {
		return fmt.Errorf("hostname on destination resolved to localhost, but migration requires an FQDN")
	}
	return nil
}

func migrationRemoteShellArgs(target migrationTarget, command string) []string {
	return append(migrationSSHArgs(target), "sh -c "+shellQuote(command))
}

func migrationHostnameLooksLocalhost(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	return hostname == "" || hostname == "localhost" || hostname == "localhost.localdomain" ||
		hostname == "127.0.0.1" || hostname == "::1" || strings.HasPrefix(hostname, "127.")
}

func migrationTargetHostnameLocalhost(message string) bool {
	lower := strings.ToLower(strings.Join(strings.Fields(message), " "))
	if strings.Contains(lower, "hostname on destination resolved to localhost") && strings.Contains(lower, "migration requires an fqdn") {
		return true
	}
	for _, field := range strings.Fields(lower) {
		field = strings.Trim(field, "[](),;")
		if strings.HasPrefix(field, "127.") {
			return true
		}
		switch field {
		case "localhost", "::1":
			return true
		}
	}
	return false
}

type migrationTarget struct {
	Username string
	Host     string
	Port     string
}

func migrationSSHTarget(destinationURI string, usernameOverride string) (migrationTarget, error) {
	parsed, err := url.Parse(strings.TrimSpace(destinationURI))
	if err != nil || !isQemuSSHMigrationURI(destinationURI) {
		return migrationTarget{}, fmt.Errorf("only qemu+ssh migration uri supports ssh key setup")
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return migrationTarget{}, fmt.Errorf("migration target host is required")
	}
	username := strings.TrimSpace(usernameOverride)
	if username == "" && parsed.User != nil {
		username = strings.TrimSpace(parsed.User.Username())
	}
	if username == "" {
		username = currentUsername()
	}
	if username == "" {
		username = "root"
	}
	port := strings.TrimSpace(parsed.Port())
	if port == "" {
		port = "22"
	}
	return migrationTarget{Username: username, Host: host, Port: port}, nil
}

func currentUsername() string {
	current, err := user.Current()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(current.Username)
}

func (p *VirshProvider) ensureMigrationPublicKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("current user home is unavailable")
	}
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", err
	}
	keyPath := filepath.Join(sshDir, "id_ed25519")
	publicKeyPath := keyPath + ".pub"
	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		if _, err := p.outputWithTimeout(15*time.Second, "ssh-keygen", "-t", "ed25519", "-N", "", "-f", keyPath); err != nil {
			return "", err
		}
	}
	content, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return "", err
	}
	publicKey := strings.TrimSpace(string(content))
	if publicKey == "" {
		return "", fmt.Errorf("ssh public key is empty")
	}
	return publicKey, nil
}

func installMigrationPublicKey(target migrationTarget, password string, publicKey string) error {
	client, err := ssh.Dial("tcp", net.JoinHostPort(target.Host, target.Port), &ssh.ClientConfig{
		User:            target.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         migrationConnectionTimeout,
	})
	if err != nil {
		return err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	return session.Run(migrationInstallKeyCommand(publicKey))
}

func migrationInstallKeyCommand(publicKey string) string {
	quotedKey := shellSingleQuote(publicKey)
	return "umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; " +
		"grep -qxF " + quotedKey + " ~/.ssh/authorized_keys || printf '%s\\n' " + quotedKey + " >> ~/.ssh/authorized_keys; " +
		"chmod 700 ~/.ssh; chmod 600 ~/.ssh/authorized_keys"
}

func (p *VirshProvider) rememberMigrationHostKey(target migrationTarget) error {
	out, err := p.outputWithTimeout(10*time.Second, "ssh-keyscan", "-p", target.Port, "-H", target.Host)
	if err != nil || strings.TrimSpace(out) == "" {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return err
	}
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return err
	}
	knownHosts := filepath.Join(sshDir, "known_hosts")
	file, err := os.OpenFile(knownHosts, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(strings.TrimSpace(out) + "\n")
	return err
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
