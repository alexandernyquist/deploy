package main

import (
	"bytes"
	"code.google.com/p/go.crypto/ssh"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type (
	ServerConfig struct {
		Name         string              `json:"name"`
		Environments []ServerEnvironment `json:"envs"`
	}

	ServerEnvironment struct {
		Name    string `json:"name"`
		Server  string `json:"server"`
		User    string `json:"user"`
		Pass    string `json:"pass"`
		Dir     string `json:"dir"`
		Command string `json:"command"`
	}

	ServerList struct {
		Servers []ServerConfig `json:"servers"`
	}
)

// Executes the actual deployment.
func executeDeployment(env ServerEnvironment) (string, error) {
	clientConfig := &ssh.ClientConfig{
		User: env.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(env.Pass),
		},
	}
	client, err := ssh.Dial("tcp", env.Server, clientConfig)
	if err != nil {
		return "", err
	}

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b

	// Execute command
	remoteCommand := fmt.Sprintf("cd %s && %s", env.Dir, env.Command)
	if err := session.Run(remoteCommand); err != nil {
		return "", err
	}

	return b.String(), nil
}

// Helper function to be able to pass strings with
// server and environment names around.
func deploy(serverList ServerList, serverName, environment string) (string, error) {
	for _, s := range serverList.Servers {
		if s.Name == serverName {
			for _, e := range s.Environments {
				if e.Name == environment {
					// Found correct server and environment
					return executeDeployment(e)
				}
			}
		}
	}

	return "", errors.New(fmt.Sprintf("No such server or environment found (%s, %s)", serverName, environment))
}

// Returns the path to the executable. Borrowed from
// https://github.com/gyepisam/redux/blob/master/redux/install.go#L90
func executingDir() (name string, err error) {
	name = os.Args[0]

	if name[0] == '.' {
		name, err = filepath.Abs(name)
		if err == nil {
			name = filepath.Clean(name)
		}
	} else {
		name, err = exec.LookPath(filepath.Clean(name))
	}

	return filepath.Dir(name), err
}

func main() {
	fmt.Println("Deploy v0.0.1")

	// Parse CLI flags
	serverName := flag.String("server", "", "Server to deploy")
	envName := flag.String("env", "", "Environment to deploy")
	flag.Parse()

	if *serverName == "" || *envName == "" {
		fmt.Println("Usage: deploy --server=serverName --env=envName")
		fmt.Println("Example: deploy --server=testserver --env=prod")
		os.Exit(1)
	}

	// Find path to config file
	var configFile string
	dir, err := executingDir()
	if err != nil {
		configFile = "servers.json"
	} else {
		configFile = filepath.Join(dir, "servers.json")
	}

	// Load configuration
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Printf("Could not open servers.json: %s (tried with path %s)\n", err, configFile)
		os.Exit(1)
	}

	var serverList ServerList
	json.Unmarshal(file, &serverList)

	// Try deploy
	fmt.Printf("Initiating deployment of %s:%s\n", *serverName, *envName)
	output, err := deploy(serverList, *serverName, *envName)
	if err != nil {
		fmt.Printf("Error while deploying: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("Deployment succeeded!")
	fmt.Printf("Output:\n%s\n", output)
}
