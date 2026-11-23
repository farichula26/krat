package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshDialer reaches the database through an SSH connection it owns: no local port, a keepalive
// that drops a dead session (a laptop sleep, a network blip) and a fresh session on the next dial.
type sshDialer struct {
	addr string // host:port of the SSH server
	cfg  *ssh.ClientConfig
	mu   sync.Mutex
	c    *ssh.Client
}

func newSSHDialer(target, keyPath string) (*sshDialer, error) {
	user, host, ok := strings.Cut(target, "@")
	if !ok {
		return nil, fmt.Errorf("DB_SSH debe ser usuario@host[:puerto], no %q", target)
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, "22")
	}
	home, _ := os.UserHomeDir()
	if rest, ok := strings.CutPrefix(keyPath, "~/"); ok {
		keyPath = filepath.Join(home, rest)
	}
	pem, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("DB_SSH_KEY: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		return nil, fmt.Errorf("DB_SSH_KEY: %w", err)
	}
	hostKeys, err := knownhosts.New(filepath.Join(home, ".ssh", "known_hosts"))
	if err != nil {
		return nil, fmt.Errorf("known_hosts (conéctate una vez con ssh para registrar el servidor): %w", err)
	}
	return &sshDialer{addr: host, cfg: &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeys,
		Timeout:         10 * time.Second,
	}}, nil
}

func (d *sshDialer) Dial(ctx context.Context, _, addr string) (net.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	// A failure on a reused session may just mean it died: retry once on a fresh one.
	for fresh := d.c == nil; ; fresh = true {
		if d.c == nil {
			c, err := ssh.Dial("tcp", d.addr, d.cfg)
			if err != nil {
				return nil, fmt.Errorf("túnel SSH: %w", err)
			}
			log.Printf("carta: túnel SSH abierto con %s", d.addr)
			d.c = c
			go keepalive(c)
		}
		conn, err := d.c.DialContext(ctx, "tcp", addr)
		if err == nil {
			return conn, nil
		}
		d.c.Close()
		d.c = nil
		if fresh || ctx.Err() != nil {
			return nil, err
		}
	}
}

// close drops the current SSH session, as a dead network would.
func (d *sshDialer) close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.c != nil {
		d.c.Close()
	}
}

// keepalive closes c when the server stops answering; Wait then returns and the pool's
// connections over it fail fast instead of hanging until their deadline.
func keepalive(c *ssh.Client) {
	go func() {
		for {
			time.Sleep(15 * time.Second)
			done := make(chan error, 1)
			go func() { _, _, err := c.SendRequest("keepalive@openssh.com", true, nil); done <- err }()
			select {
			case err := <-done:
				if err == nil {
					continue
				}
			case <-time.After(10 * time.Second):
			}
			c.Close()
			return
		}
	}()
	err := c.Wait()
	log.Printf("carta: túnel SSH cerrado (%v)", err)
}

// openCarta connects lazily to the menu database, through SSH when sshTarget is set.
func openCarta(ctx context.Context, databaseURL, sshTarget, keyPath string) (*pgxpool.Pool, *sshDialer, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, nil, err
	}
	var d *sshDialer
	if sshTarget != "" {
		if d, err = newSSHDialer(sshTarget, keyPath); err != nil {
			return nil, nil, err
		}
		cfg.ConnConfig.DialFunc = d.Dial
	}
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	return db, d, err
}
