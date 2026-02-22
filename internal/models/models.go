package models

import "time"

type Node struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	Username   string    `json:"username"`
	PrivateKey string    `json:"private_key,omitempty"`
	Status     string    `json:"status"`
	IsLocal    bool      `json:"is_local"`
	CreatedAt  time.Time `json:"created_at"`
}

type Application struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DockerImage string    `json:"docker_image"`
	EnvVars     string    `json:"env_vars"` // JSON {"KEY":"VAL"}
	Ports       string    `json:"ports"`    // JSON ["8080:80"]
	Command     string    `json:"command"`
	GithubRepo  string    `json:"github_repo"`
	CreatedAt   time.Time `json:"created_at"`
}

type Deployment struct {
	ID            string    `json:"id"`
	ApplicationID string    `json:"application_id"`
	NodeID        string    `json:"node_id"`
	ContainerName string    `json:"container_name"`
	ContainerID   string    `json:"container_id"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	// Joined fields
	AppName     string `json:"app_name,omitempty"`
	NodeName    string `json:"node_name,omitempty"`
	DockerImage string `json:"docker_image,omitempty"`
}
