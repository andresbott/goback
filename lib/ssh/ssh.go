package ssh

import (
	"errors"
	"fmt"
	"github.com/AndresBott/goback/internal/profile"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type AuthType int

const (
	Password AuthType = iota
	PrivateKey
	SshAgent
)

type Cfg struct {
	Host          string
	Port          int
	Auth          AuthType
	User          string
	Password      string
	PrivateKey    string
	PassPhrase    string
	IgnoreHostKey bool
}

type Client struct {
	config    *ssh.ClientConfig
	server    string
	conn      *ssh.Client
	agentConn net.Conn
}

func New(cfg Cfg) (*Client, error) {

	if cfg.User == "" {
		return nil, errors.New("user is a mandatory field")
	}

	var authMethod []ssh.AuthMethod
	var sshAgentConnection net.Conn

	switch cfg.Auth {
	// => Password authentication
	case Password:
		authMethod = append(authMethod, ssh.Password(cfg.Password))

	// => Private key authentication
	case PrivateKey:
		pKey, err := readKey(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("unable to read private key: %v", err)
		}
		var signer ssh.Signer

		if cfg.PassPhrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pKey, []byte(cfg.PassPhrase))
			if err != nil {
				return nil, fmt.Errorf("unable to parse private key: %v", err)
			}
		} else {
			signer, err = ssh.ParsePrivateKey(pKey)
			if err != nil {
				return nil, fmt.Errorf("unable to parse private key: %v", err)
			}
		}

		authMethod = append(authMethod, ssh.PublicKeys(signer))

	// => private key in ssh agent
	case SshAgent:
		signers, sshAgCon, err := sshAgentSigners()
		if err != nil {
			return nil, fmt.Errorf("unable to get keys from ssh agent: %v", err)
		}
		sshAgentConnection = *sshAgCon

		for _, s := range signers {
			authMethod = append(authMethod, ssh.PublicKeys(s))
		}

	default:
		return nil, errors.New("authentication type not provided")
	}

	var knownHostCallback func(hostname string, remote net.Addr, key ssh.PublicKey) error
	if cfg.IgnoreHostKey {
		knownHostCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		}
	} else {

		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting users home dir: %v", err)
		}

		knownHostsAbsFile, err := filepath.Abs(filepath.Join(userHome, ".ssh/known_hosts"))
		if err != nil {
			return nil, fmt.Errorf("absolute path for known_hosts file: %v", err)
		}
		fn, err := knownhosts.New(knownHostsAbsFile)
		if err != nil {
			return nil, fmt.Errorf("error checking known_hosts: %v", err)
		}
		knownHostCallback = fn
	}

	config := &ssh.ClientConfig{
		User:            cfg.User,
		HostKeyCallback: knownHostCallback,
		Auth:            authMethod,
	}

	client := &Client{
		config:    config,
		server:    fmt.Sprintf("%v:%v", cfg.Host, cfg.Port),
		agentConn: sshAgentConnection,
	}
	return client, nil
}

// readKey will try to read the private key from the location passed, using the default location as fallback
// and return the key as byte array
func readKey(path string) ([]byte, error) {
	var err error
	var pemBytes []byte

	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting users home dir: %v", err)
	}

	keyFile, err := filepath.Abs(filepath.Join(userHome, ".ssh/id_rsa"))
	if err != nil {
		return nil, fmt.Errorf("absolute path for default id_rsa file: %v", err)
	}

	if path != "" {
		keyFile = path
	}

	keyFile, err = filepath.Abs(keyFile)
	if err != nil {
		return nil, fmt.Errorf("absolute path for private key %v", err)
	}

	// check for private key
	if _, err := os.Stat(keyFile); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("private key does not exists")
	}

	// read private key file
	// #nosec G304 - input comes from config file
	pemBytes, err = os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("reading private key file failed %v", err)
	}

	return pemBytes, nil

}

// sshAgentKeys will try to connect to the the current ssh agent and retrieve the keys from there
func sshAgentSigners() ([]ssh.Signer, *net.Conn, error) {
	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAuthSock == "" {
		return nil, nil, errors.New("env variable SSH_AUTH_SOCK is not set or ssh agent not running")
	}

	conn, err := net.Dial("unix", sshAuthSock)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to connect to ssh agent: %v", err)
	}

	sshAgentClient := agent.NewClient(conn)

	signers, err := sshAgentClient.Signers()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting ssh signers from agent: %s", err)
	}
	return signers, &conn, err
}

func (sshc *Client) Connect() error {
	if sshc.conn != nil {
		return errors.New("connection already open")
	}

	// open connection
	conn, err := ssh.Dial("tcp", sshc.server, sshc.config)
	if err != nil {
		return fmt.Errorf("dial to %v failed %v", sshc.server, err)
	}
	sshc.conn = conn

	return nil
}

func GetAuthType(in profile.AuthType) AuthType {
	switch in {
	case "sshPassword":
		return Password
	case "sshKey":
		return PrivateKey
	case "sshAgent":
		return SshAgent
	default:
		return Password
	}
}

func (sshc *Client) Disconnect() error {
	if sshc.agentConn != nil {
		_ = sshc.agentConn.Close()
	}

	err := sshc.conn.Close()
	sshc.conn = nil
	return err
}

func (sshc *Client) Connection() *ssh.Client {
	return sshc.conn
}

func (sshc *Client) Session() (*ssh.Session, error) {
	if sshc.conn == nil {
		return nil, errors.New("unable to create session: connection not open")
	}

	// open session
	session, err := sshc.conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("create session for %v failed %v", sshc.server, err)
	}
	return session, nil
}

// Which identifies the path of a binary in the path on the remote machine.
func (sshc *Client) Which(app string) (out string, err error) {
	session, err := sshc.Session()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer func() {
		// we ignore the EOF error on close since it is expected if session was closed by wait()
		if cErr := session.Close(); cErr != nil && !errors.Is(cErr, io.EOF) {
			err = errors.Join(err, cErr)
		}
	}()

	cmd := fmt.Sprintf("which %s", app)
	output, err := session.CombinedOutput(cmd)

	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("command '%s' failed with exit code %d", cmd, exitErr.ExitStatus())
		}
		return "", fmt.Errorf("SSH error running '%s': %w", cmd, err)
	}

	return strings.Trim(string(output), " \n"), nil
}
