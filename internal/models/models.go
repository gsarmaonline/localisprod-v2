package models

import "time"

type User struct {
	ID        string    `json:"id"`
	GoogleID  string    `json:"google_id,omitempty"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Node struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Host           string    `json:"host"`
	Port           int       `json:"port"`
	Username       string    `json:"username"`
	PrivateKey     string    `json:"private_key,omitempty"`
	Status         string    `json:"status"`
	IsLocal        bool      `json:"is_local"`
	TraefikEnabled bool      `json:"traefik_enabled"`
	UserID         string    `json:"user_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type Application struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	DockerImage    string    `json:"docker_image"`
	DockerfilePath string    `json:"dockerfile_path"`
	EnvVars        string    `json:"env_vars"`   // JSON {"KEY":"VAL"}
	Ports          string    `json:"ports"`      // JSON ["8080:80"]
	Command        string    `json:"command"`
	GithubRepo     string    `json:"github_repo"`
	Domain         string    `json:"domain"`
	Databases      string    `json:"databases"`  // JSON ["db-id-1"]
	Caches         string    `json:"caches"`     // JSON ["cache-id-1"]
	Kafkas         string    `json:"kafkas"`     // JSON ["kafka-id-1"]
	CreatedAt      time.Time `json:"created_at"`
}

type Cache struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	NodeID        string    `json:"node_id"`
	Password      string    `json:"password,omitempty"`
	Port          int       `json:"port"`
	ContainerName string    `json:"container_name"`
	Status        string    `json:"status"`
	UserID        string    `json:"user_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	// Joined fields
	NodeHost string `json:"node_host,omitempty"`
	NodeName string `json:"node_name,omitempty"`
}

type Kafka struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	NodeID        string    `json:"node_id"`
	Port          int       `json:"port"`
	ContainerName string    `json:"container_name"`
	Status        string    `json:"status"`
	UserID        string    `json:"user_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	// Joined fields
	NodeHost string `json:"node_host,omitempty"`
	NodeName string `json:"node_name,omitempty"`
}

type Database struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`    // postgres, mysql, redis, mongodb
	Version       string    `json:"version"`
	NodeID        string    `json:"node_id"`
	DBName        string    `json:"dbname"`
	DBUser        string    `json:"db_user"`
	Password      string    `json:"password,omitempty"`
	Port          int       `json:"port"`
	ContainerName string    `json:"container_name"`
	Status        string    `json:"status"`
	UserID        string    `json:"user_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	// Joined fields
	NodeHost string `json:"node_host,omitempty"`
	NodeName string `json:"node_name,omitempty"`
}

type Deployment struct {
	ID            string    `json:"id"`
	ApplicationID string    `json:"application_id"`
	NodeID        string    `json:"node_id"`
	ContainerName string    `json:"container_name"`
	ContainerID   string    `json:"container_id"`
	Status        string    `json:"status"`
	UserID        string    `json:"user_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	// Joined fields
	AppName     string `json:"app_name,omitempty"`
	NodeName    string `json:"node_name,omitempty"`
	DockerImage string `json:"docker_image,omitempty"`
}
