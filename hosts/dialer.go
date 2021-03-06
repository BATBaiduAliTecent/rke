package hosts

import (
	"fmt"
	"net"
	"net/http"

	"golang.org/x/crypto/ssh"
)

type DialerFactory func(h *Host) (func(network, address string) (net.Conn, error), error)

type dialer struct {
	host   *Host
	signer ssh.Signer
}

func SSHFactory(h *Host) (func(network, address string) (net.Conn, error), error) {
	key, err := checkEncryptedKey(h.SSHKey, h.SSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse the private key: %v", err)
	}
	dialer := &dialer{
		host:   h,
		signer: key,
	}
	return dialer.Dial, nil
}

func (d *dialer) Dial(network, addr string) (net.Conn, error) {
	sshAddr := d.host.Address + ":22"
	// Build SSH client configuration
	cfg, err := makeSSHConfig(d.host.User, d.signer)
	if err != nil {
		return nil, fmt.Errorf("Error configuring SSH: %v", err)
	}
	// Establish connection with SSH server
	conn, err := ssh.Dial("tcp", sshAddr, cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to dial ssh using address [%s]: %v", sshAddr, err)
	}
	if len(d.host.DockerSocket) == 0 {
		d.host.DockerSocket = "/var/run/docker.sock"
	}
	remote, err := conn.Dial("unix", d.host.DockerSocket)
	if err != nil {
		return nil, fmt.Errorf("Failed to dial to Docker socket: %v", err)
	}
	return remote, err
}

func (h *Host) newHTTPClient(dialerFactory DialerFactory) (*http.Client, error) {
	var factory DialerFactory

	if dialerFactory == nil {
		factory = SSHFactory
	} else {
		factory = dialerFactory
	}

	dialer, err := factory(h)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: &http.Transport{
			Dial: dialer,
		},
	}, nil
}
