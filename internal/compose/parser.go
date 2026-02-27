package compose

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---- raw docker-compose structures ----

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image       string        `yaml:"image"`
	Build       interface{}   `yaml:"build"`       // string or map{context:...}
	Ports       []interface{} `yaml:"ports"`       // "host:container" or map
	Environment interface{}   `yaml:"environment"` // map or []"KEY=VAL"
	Command     interface{}   `yaml:"command"`     // string or []string
	Volumes     []string      `yaml:"volumes"`
	DependsOn   interface{}   `yaml:"depends_on"`  // []string or map
}

// ---- preview types returned to the client ----

type ParsedApplication struct {
	Name        string            `json:"name"`
	DockerImage string            `json:"docker_image"`
	BuildPath   string            `json:"build_path,omitempty"`
	Ports       []string          `json:"ports"`
	Volumes     []string          `json:"volumes"`
	EnvVars     map[string]string `json:"env_vars"`
	Command     string            `json:"command,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
}

type ParsedDatabase struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`    // postgres, redis
	Version string            `json:"version"`
	Port    int               `json:"port"`
	DBName  string            `json:"dbname,omitempty"`
	DBUser  string            `json:"db_user,omitempty"`
	EnvVars map[string]string `json:"env_vars"`
	Volumes []string          `json:"volumes"`
}

type ParsedCache struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Port    int      `json:"port"`
	Volumes []string `json:"volumes"`
}

type ParsedKafka struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Port    int    `json:"port"`
}

type ParsedObjectStorage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Port    int    `json:"port"`
}

type Preview struct {
	Applications  []ParsedApplication  `json:"applications"`
	Databases     []ParsedDatabase     `json:"databases"`
	Caches        []ParsedCache        `json:"caches"`
	Kafkas        []ParsedKafka        `json:"kafkas"`
	ObjectStorages []ParsedObjectStorage `json:"object_storages"`
}

// Parse parses the given docker-compose YAML content and returns a Preview.
func Parse(content []byte) (*Preview, error) {
	var cf composeFile
	if err := yaml.Unmarshal(content, &cf); err != nil {
		return nil, fmt.Errorf("invalid docker-compose YAML: %w", err)
	}
	if cf.Services == nil {
		return nil, fmt.Errorf("no services found in docker-compose file")
	}

	preview := &Preview{}

	for name, svc := range cf.Services {
		env := parseEnv(svc.Environment)
		ports := parsePorts(svc.Ports)
		deps := parseDependsOn(svc.DependsOn)
		image, version := splitImageTag(svc.Image)
		buildPath := parseBuild(svc.Build)

		kind := classify(name, image, svc.Build)

		switch kind {
		case "database_postgres":
			db := ParsedDatabase{
				Name:    name,
				Type:    "postgres",
				Version: version,
				Port:    firstHostPort(ports, 5432),
				Volumes: svc.Volumes,
				EnvVars: env,
			}
			if v, ok := env["POSTGRES_DB"]; ok {
				db.DBName = v
			} else {
				db.DBName = name
			}
			if v, ok := env["POSTGRES_USER"]; ok {
				db.DBUser = v
			} else {
				db.DBUser = name
			}
			preview.Databases = append(preview.Databases, db)

		case "database_redis":
			// redis with named volumes → treat as persistent database
			db := ParsedDatabase{
				Name:    name,
				Type:    "redis",
				Version: version,
				Port:    firstHostPort(ports, 6379),
				Volumes: svc.Volumes,
				EnvVars: env,
			}
			preview.Databases = append(preview.Databases, db)

		case "cache":
			preview.Caches = append(preview.Caches, ParsedCache{
				Name:    name,
				Version: version,
				Port:    firstHostPort(ports, 6379),
				Volumes: svc.Volumes,
			})

		case "kafka":
			preview.Kafkas = append(preview.Kafkas, ParsedKafka{
				Name:    name,
				Version: version,
				Port:    firstHostPort(ports, 9092),
			})

		case "object_storage":
			preview.ObjectStorages = append(preview.ObjectStorages, ParsedObjectStorage{
				Name:    name,
				Version: version,
				Port:    firstHostPort(ports, 9000),
			})

		default: // application
			app := ParsedApplication{
				Name:      name,
				DependsOn: deps,
				Ports:     ports,
				Volumes:   svc.Volumes,
				EnvVars:   env,
				Command:   parseCommand(svc.Command),
			}
			if buildPath != "" {
				app.DockerImage = ""
				app.BuildPath = buildPath
			} else {
				app.DockerImage = svc.Image
			}
			preview.Applications = append(preview.Applications, app)
		}
	}

	return preview, nil
}

// classify determines the service kind based on image name and service name heuristics.
func classify(name, image string, build interface{}) string {
	img := strings.ToLower(image)
	svcName := strings.ToLower(name)

	// Build-based services are always applications
	if build != nil {
		return "application"
	}

	// Image-based classification
	switch {
	case strings.HasPrefix(img, "postgres") || strings.HasPrefix(img, "postgresql"):
		return "database_postgres"

	case strings.HasPrefix(img, "redis"):
		// Heuristic: "cache" in name → cache, otherwise persistent DB
		if strings.Contains(svcName, "cache") {
			return "cache"
		}
		return "database_redis"

	case strings.HasPrefix(img, "confluentinc/cp-kafka") ||
		strings.HasPrefix(img, "bitnami/kafka") ||
		strings.HasPrefix(img, "apache/kafka") ||
		strings.HasPrefix(img, "wurstmeister/kafka"):
		return "kafka"

	case strings.HasPrefix(img, "minio/minio") ||
		strings.HasPrefix(img, "quay.io/minio") ||
		strings.Contains(img, "garage"):
		return "object_storage"
	}

	// Name-based fallback heuristics
	switch {
	case strings.Contains(svcName, "postgres") || strings.Contains(svcName, "postgresql") ||
		svcName == "db" || svcName == "database":
		return "database_postgres"
	case strings.Contains(svcName, "redis") && strings.Contains(svcName, "cache"):
		return "cache"
	case strings.Contains(svcName, "kafka"):
		return "kafka"
	case strings.Contains(svcName, "minio") || strings.Contains(svcName, "storage"):
		return "object_storage"
	}

	return "application"
}

// splitImageTag splits "image:tag" into (image, tag). Tag defaults to "latest".
func splitImageTag(image string) (string, string) {
	// Handle digests like image@sha256:... — treat as no version
	if i := strings.LastIndex(image, ":"); i > 0 {
		// Make sure it's not a port in a registry URL (host:port/image)
		rest := image[i+1:]
		if !strings.Contains(rest, "/") {
			return image[:i], rest
		}
	}
	return image, "latest"
}

// parseBuild returns the build context path from the build field.
func parseBuild(build interface{}) string {
	if build == nil {
		return ""
	}
	switch v := build.(type) {
	case string:
		return v
	case map[string]interface{}:
		if ctx, ok := v["context"].(string); ok {
			return ctx
		}
	}
	return "."
}

// parseEnv handles both map and list forms of docker-compose environment.
func parseEnv(env interface{}) map[string]string {
	result := map[string]string{}
	if env == nil {
		return result
	}
	switch v := env.(type) {
	case map[string]interface{}:
		for k, val := range v {
			if val == nil {
				result[k] = ""
			} else {
				result[k] = fmt.Sprintf("%v", val)
			}
		}
	case []interface{}:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			idx := strings.IndexByte(s, '=')
			if idx < 0 {
				result[s] = ""
			} else {
				result[s[:idx]] = s[idx+1:]
			}
		}
	}
	return result
}

// parsePorts normalises port specs to "hostPort:containerPort" strings.
func parsePorts(ports []interface{}) []string {
	var result []string
	for _, p := range ports {
		switch v := p.(type) {
		case string:
			result = append(result, v)
		case int:
			s := strconv.Itoa(v)
			result = append(result, s+":"+s)
		case map[string]interface{}:
			published, _ := v["published"].(int)
			target, _ := v["target"].(int)
			if published != 0 && target != 0 {
				result = append(result, fmt.Sprintf("%d:%d", published, target))
			}
		}
	}
	return result
}

// parseCommand converts string or []string command to a single string.
func parseCommand(cmd interface{}) string {
	if cmd == nil {
		return ""
	}
	switch v := cmd.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, p := range v {
			parts = append(parts, fmt.Sprintf("%v", p))
		}
		return strings.Join(parts, " ")
	}
	return ""
}

// parseDependsOn handles both []string and map forms of depends_on.
func parseDependsOn(dep interface{}) []string {
	if dep == nil {
		return nil
	}
	var result []string
	switch v := dep.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
	case map[string]interface{}:
		for k := range v {
			result = append(result, k)
		}
	}
	return result
}

// firstHostPort extracts the first host port from port mappings, returning fallback if none found.
func firstHostPort(ports []string, fallback int) int {
	for _, p := range ports {
		host := p
		if idx := strings.LastIndex(p, ":"); idx >= 0 {
			host = p[:idx]
		}
		// Strip IPv6 bracket prefix if any
		host = strings.TrimPrefix(host, "[::1]:")
		host = strings.TrimPrefix(host, "0.0.0.0:")
		if n, err := strconv.Atoi(host); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
