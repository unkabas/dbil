package postgres

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/unkabas/dbil/internal/store"
)

// sshDialTimeout bounds the TCP+handshake to the bastion.
const sshDialTimeout = 10 * time.Second

// sshTunnel wraps an ssh.Client used to reach a Postgres instance whose port
// is only reachable from the bastion.
type sshTunnel struct {
	client *ssh.Client
}

// openTunnel establishes an SSH client to the revealed host. It returns the
// tunnel and the server's host-key fingerprint (SHA256). When the host already
// has a pinned fingerprint, a mismatch is a hard error; otherwise the
// fingerprint is returned so the caller can pin it (trust on first use).
func openTunnel(ctx context.Context, rev store.RevealedSSHHost) (*sshTunnel, string, error) {
	auth, err := sshAuthMethods(rev)
	if err != nil {
		return nil, "", err
	}
	var observed string
	hostKeyCallback := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		observed = ssh.FingerprintSHA256(key)
		if rev.HostKeyFingerprint != "" && rev.HostKeyFingerprint != observed {
			return fmt.Errorf("ssh host key mismatch: pinned %q, server presented %q", rev.HostKeyFingerprint, observed)
		}
		return nil
	}
	cfg := &ssh.ClientConfig{
		User:            rev.Username,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         sshDialTimeout,
	}
	addr := net.JoinHostPort(rev.Host, strconv.Itoa(rev.Port))

	dialer := net.Dialer{Timeout: sshDialTimeout}
	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, "", fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, addr, cfg)
	if err != nil {
		_ = netConn.Close()
		return nil, observed, fmt.Errorf("ssh handshake %s: %w", addr, err)
	}
	return &sshTunnel{client: ssh.NewClient(sshConn, chans, reqs)}, observed, nil
}

// sshAuthMethods builds the auth methods for the host's configured method.
func sshAuthMethods(rev store.RevealedSSHHost) ([]ssh.AuthMethod, error) {
	switch rev.AuthMethod {
	case store.SSHAuthPassword:
		return []ssh.AuthMethod{ssh.Password(rev.Secret)}, nil
	case store.SSHAuthKey:
		var (
			signer ssh.Signer
			err    error
		)
		if rev.KeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(rev.Secret), []byte(rev.KeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(rev.Secret))
		}
		if err != nil {
			return nil, fmt.Errorf("parse ssh private key: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		return nil, fmt.Errorf("unknown ssh auth method %q", rev.AuthMethod)
	}
}

// DialContext dials addr through the tunnel. pgx supplies (network, addr) for
// the Postgres backend; ssh.Client.Dial is not context-aware, so cancellation
// is honoured by racing the dial against ctx.Done().
func (t *sshTunnel) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := t.client.Dial(network, addr)
		ch <- result{c, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("tunnel dial %s: %w", addr, r.err)
		}
		return r.conn, nil
	}
}

// Close tears down the SSH client (and all tunnelled connections).
func (t *sshTunnel) Close() error {
	if t == nil || t.client == nil {
		return nil
	}
	return t.client.Close()
}
