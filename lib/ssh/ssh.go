package ssh

import (
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"net"
	"os"
	"path/filepath"
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
	config *ssh.ClientConfig
	server string
	conn   *ssh.Client
}

const (
	knownHostsFile    = "~/.ssh/known_hosts"
	DefaultPrivateKey = "~/.ssh/id_rsa"
)

func New(cfg Cfg) (*Client, error) {

	if cfg.User == "" {
		return nil, errors.New("user is a mandatory field")
	}

	var authMethod []ssh.AuthMethod

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
		keys, err := sshAgentKeys()
		if err != nil {
			return nil, fmt.Errorf("unable to get keys from ssh agent: %v", err)
		}

		for _, k := range keys {
			signer, err := ssh.ParsePrivateKey(k.Blob)
			if err != nil {
				return nil, fmt.Errorf("unable to parse private key: %v", err)
			}
			authMethod = append(authMethod, ssh.PublicKeys(signer))
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
		fn, err := knownhosts.New(knownHostsFile)
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
		config: config,
		server: fmt.Sprintf("%v:%v", cfg.Host, cfg.Port),
	}
	return client, nil
}

// readKey will try to read the private key from the location passed, using the default location as fallback
// and return the key as byte array
func readKey(path string) ([]byte, error) {
	var err error
	var pemBytes []byte

	keyFile := DefaultPrivateKey
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
	pemBytes, err = os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("reading private key file failed %v", err)
	}

	return pemBytes, nil

}

// sshAgentKeys will try to connect to the the current ssh agent and retrieve the keys from there
func sshAgentKeys() ([]*agent.Key, error) {
	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAuthSock == "" {
		return nil, errors.New("env variable SSH_AUTH_SOCK is not set or ssh agent not running")
	}

	conn, err := net.Dial("unix", sshAuthSock)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to ssh agent: %v", err)
	}
	defer conn.Close()

	sshAgentClient := agent.NewClient(conn)
	loadedKeys, err := sshAgentClient.List()
	if err != nil {
		return nil, fmt.Errorf("error listing keys: %s", err)
	}
	return loadedKeys, err
}

func (scpc *Client) Connect() error {
	if scpc.conn != nil {
		return errors.New("connection already open")
	}

	// open connection
	conn, err := ssh.Dial("tcp", scpc.server, scpc.config)
	if err != nil {
		return fmt.Errorf("dial to %v failed %v", scpc.server, err)
	}
	scpc.conn = conn

	return nil
}

func GetAuthType(in string) AuthType {
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

func (scpc *Client) Disconnect() error {
	//s.scpClient.Close()

	err := scpc.conn.Close()
	scpc.conn = nil
	return err
}

func (scpc *Client) Connection() *ssh.Client {
	return scpc.conn
}

func (scpc *Client) Session() (*ssh.Session, error) {
	if scpc.conn == nil {
		return nil, errors.New("unable to create session: connection not open")
	}

	// open session
	session, err := scpc.conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("create session for %v failed %v", scpc.server, err)
	}
	return session, nil
}
