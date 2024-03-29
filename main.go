package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/badjware/gitlabfs/fs"
	"github.com/badjware/gitlabfs/git"
	"github.com/badjware/gitlabfs/gitlab"
	"gopkg.in/yaml.v2"
)

type (
	Config struct {
		FS     FSConfig     `yaml:"fs,omitempty"`
		Gitlab GitlabConfig `yaml:"gitlab,omitempty"`
		Git    GitConfig    `yaml:"git,omitempty"`
	}
	FSConfig struct {
		Mountpoint   string `yaml:"mountpoint,omitempty"`
		MountOptions string `yaml:"mountoptions,omitempty"`
	}
	GitlabConfig struct {
		URL                string `yaml:"url,omitempty"`
		Token              string `yaml:"token,omitempty"`
		GroupIDs           []int  `yaml:"group_ids,omitempty"`
		UserIDs            []int  `yaml:"user_ids,omitempty"`
		IncludeCurrentUser bool   `yaml:"include_current_user,omitempty"`
	}
	GitConfig struct {
		CloneLocation    string `yaml:"clone_location,omitempty"`
		Remote           string `yaml:"remote,omitempty"`
		PullMethod       string `yaml:"pull_method,omitempty"`
		OnClone          string `yaml:"on_clone,omitempty"`
		AutoPull         bool   `yaml:"auto_pull,omitempty"`
		Depth            int    `yaml:"depth,omitempty"`
		QueueSize        int    `yaml:"queue_size,omitempty"`
		QueueWorkerCount int    `yaml:"worker_count,omitempty"`
	}
)

func loadConfig(configPath string) (*Config, error) {
	// defaults
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(os.Getenv("HOME"), ".local/share")
	}
	defaultCloneLocation := filepath.Join(dataHome, "gitlabfs")

	config := &Config{
		FS: FSConfig{
			Mountpoint:   "",
			MountOptions: "nodev,nosuid",
		},
		Gitlab: GitlabConfig{
			URL:                "https://gitlab.com",
			Token:              "",
			GroupIDs:           []int{9970},
			UserIDs:            []int{},
			IncludeCurrentUser: true,
		},
		Git: GitConfig{
			CloneLocation:    defaultCloneLocation,
			Remote:           "origin",
			PullMethod:       "http",
			OnClone:          "init",
			AutoPull:         false,
			Depth:            0,
			QueueSize:        200,
			QueueWorkerCount: 5,
		},
	}

	if configPath != "" {
		f, err := os.Open(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open config file: %v", err)
		}
		defer f.Close()

		d := yaml.NewDecoder(f)
		if err := d.Decode(config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %v", err)
		}
	}

	return config, nil
}

func makeGitlabConfig(config *Config) (*gitlab.GitlabClientParam, error) {
	// parse pull_method
	if config.Git.PullMethod != gitlab.PullMethodHTTP && config.Git.PullMethod != gitlab.PullMethodSSH {
		return nil, fmt.Errorf("pull_method must be either \"%v\" or \"%v\"", gitlab.PullMethodHTTP, gitlab.PullMethodSSH)
	}

	return &gitlab.GitlabClientParam{
		PullMethod:         config.Git.PullMethod,
		IncludeCurrentUser: config.Gitlab.IncludeCurrentUser && config.Gitlab.Token != "",
	}, nil
}

func makeGitConfig(config *Config) (*git.GitClientParam, error) {
	// Parse the gilab url
	parsedGitlabURL, err := url.Parse(config.Gitlab.URL)
	if err != nil {
		return nil, err
	}

	// parse on_clone
	cloneMethod := 0
	if config.Git.OnClone == "init" {
		cloneMethod = git.CloneInit
	} else if config.Git.OnClone == "clone" {
		cloneMethod = git.CloneClone
	} else {
		return nil, fmt.Errorf("on_clone must be either \"init\" or \"clone\"")
	}

	return &git.GitClientParam{
		CloneLocation:    config.Git.CloneLocation,
		RemoteName:       config.Git.Remote,
		RemoteURL:        parsedGitlabURL,
		CloneMethod:      cloneMethod,
		AutoPull:         config.Git.AutoPull,
		PullDepth:        config.Git.Depth,
		QueueSize:        config.Git.QueueSize,
		QueueWorkerCount: config.Git.QueueWorkerCount,
	}, nil
}

func main() {
	configPath := flag.String("config", "", "The config file")
	mountoptionsFlag := flag.String("o", "", "Filesystem mount options. See mount.fuse(8)")
	debug := flag.Bool("debug", false, "Enable debug logging")

	flag.Usage = func() {
		fmt.Println("USAGE:")
		fmt.Printf("    %s MOUNTPOINT\n\n", os.Args[0])
		fmt.Println("OPTIONS:")
		flag.PrintDefaults()
	}
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Configure mountpoint
	mountpoint := config.FS.Mountpoint
	if flag.NArg() == 1 {
		mountpoint = flag.Arg(0)
	}
	if mountpoint == "" {
		fmt.Println("Mountpoint is not configured in config file and missing from command-line arguments")
		flag.Usage()
		os.Exit(2)
	}

	// Configure mountoptions
	mountoptions := config.FS.MountOptions
	if *mountoptionsFlag != "" {
		mountoptions = *mountoptionsFlag
	}
	parsedMountoptions := make([]string, 0)
	if mountoptions != "" {
		parsedMountoptions = strings.Split(mountoptions, ",")
	}

	// Create the git client
	gitClientParam, err := makeGitConfig(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	gitClient, _ := git.NewClient(*gitClientParam)

	// Create the gitlab client
	gitlabClientParam, err := makeGitlabConfig(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	gitlabClient, _ := gitlab.NewClient(config.Gitlab.URL, config.Gitlab.Token, *gitlabClientParam)

	// Start the filesystem
	err = fs.Start(
		mountpoint,
		parsedMountoptions,
		&fs.FSParam{Git: gitClient, Gitlab: gitlabClient, RootGroupIds: config.Gitlab.GroupIDs, UserIds: config.Gitlab.UserIDs},
		*debug,
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
