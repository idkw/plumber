package config

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/golang/protobuf/jsonpb"
	"github.com/pkg/errors"

	"github.com/batchcorp/plumber-schemas/build/go/protos"
)

type IConfig interface {
	ConfigExists(fileName string) bool
	ReadConfig(fileName string) (*Config, error)
	WriteConfig(fileName string, data []byte) error
}

// Config stores Account IDs and the auth_token cookie
type Config struct {
	PlumberID   string                        `json:"plumber_id"`
	Token       string                        `json:"token"`
	TeamID      string                        `json:"team_id"`
	UserID      string                        `json:"user_id"`
	Connections map[string]*protos.Connection `json:"connection"`
}

// storageConfig is used to persist the config to disk. Protos can't be marshaled along with regular JSON, so we
// marshal each connection message into bytes and add them to the connections slice. The resulting JSON can then
// be marshalled normally
type storageConfig struct {
	PlumberID   string            `json:"plumber_id"`
	Token       string            `json:"token"`
	TeamID      string            `json:"team_id"`
	UserID      string            `json:"user_id"`
	Connections map[string][]byte `json:"connections"`
}

// Save is a convenience method of persisting the config to disk via a single call
func (p *Config) Save() error {
	data, err := p.Marshal()
	if err != nil {
		return err
	}

	return WriteConfig("config.json", data)
}

// Marshal marshals a Config struct to JSON
func (p *Config) Marshal() ([]byte, error) {
	cfg := &storageConfig{
		PlumberID:   p.PlumberID,
		Token:       p.Token,
		TeamID:      p.TeamID,
		UserID:      p.UserID,
		Connections: make(map[string][]byte, 0),
	}

	m := jsonpb.Marshaler{}

	// Connection proto messages need to be marshaled individually before we can marshal the entire struct
	for k, v := range p.Connections {
		buf := bytes.NewBuffer([]byte(``))
		m.Marshal(buf, v)
		cfg.Connections[k] = buf.Bytes()
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal to JSON")
	}

	return data, nil
}

// ReadConfig reads a config JSON file into a Config struct
func ReadConfig(fileName string) (*Config, error) {
	f, err := getConfigJson(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read ~/.batchsh/%s", fileName)
	}

	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read ~/.batchsh/%s", fileName)
	}

	storedCfg := &storageConfig{}
	if err := json.Unmarshal(data, storedCfg); err != nil {
		return nil, errors.Wrapf(err, "could not unmarshal ~/.batchsh/%s", fileName)
	}

	cfg := &Config{
		PlumberID:   storedCfg.PlumberID,
		Token:       storedCfg.Token,
		TeamID:      storedCfg.TeamID,
		UserID:      storedCfg.UserID,
		Connections: make(map[string]*protos.Connection, 0),
	}

	// Connection proto messages need to be un-marshaled individually
	var count int
	for k, v := range storedCfg.Connections {
		conn := &protos.Connection{}

		if err := jsonpb.Unmarshal(bytes.NewBuffer(v), conn); err != nil {
			return nil, errors.Wrapf(err, "unable to unmarshal stored connection '%s'", k)
		}

		cfg.Connections[k] = conn
		count++
	}

	logrus.Infof("Loaded '%d' stored connections", count)

	return cfg, nil
}

// Exists determines if a config file exists yet
func Exists(fileName string) bool {
	configDir, err := getConfigDir()
	if err != nil {
		return false
	}
	configPath := path.Join(configDir, fileName)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return false
	}

	return true
}

// WriteConfig writes a Batch struct as JSON into a config.json file
func WriteConfig(fileName string, data []byte) error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	configPath := path.Join(configDir, fileName)

	f, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

// getConfigJson attempts to read a user's .batchsh/config.json file to get saved credentials
func getConfigJson(fileName string) (*os.File, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}
	configPath := path.Join(configDir, fileName)

	// Directory ~/.batchsh/ doesn't exist, create it
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := createConfigDir(); err != nil {
			return nil, err
		}

		// Create ~/.batchsh/config.json
		f, err := os.Create(path.Join(configDir, fileName))
		if err != nil {
			return nil, err
		}

		f.WriteString("{}")
	}

	// Config exists, open it
	return os.Open(configPath)
}

// getConfigDir returns a directory where the batch configuration will be stored
func getConfigDir() (string, error) {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "unable to locate user's home directory")
	}

	return path.Join(homeDir, ".batchsh"), nil
}

// createConfigDir will create a json file located at ~/.batchsh/config.json to store plumber authentication credentials
func createConfigDir() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	if err := os.Mkdir(configDir, 0755); err != nil && !os.IsExist(err) {
		return errors.Wrap(err, "unable to create .batchsh directory")
	}

	return nil
}
