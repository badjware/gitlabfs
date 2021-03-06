package gitlab

import (
	"github.com/xanzy/go-gitlab"
)

type Project struct {
	ID            int
	Name          string
	CloneURL      string
	DefaultBranch string
}

func (c *gitlabClient) newProjectFromGitlabProject(project *gitlab.Project) Project {
	// https://godoc.org/github.com/xanzy/go-gitlab#Project
	p := Project{
		ID:            project.ID,
		Name:          project.Path,
		DefaultBranch: project.DefaultBranch,
	}
	if p.DefaultBranch == "" {
		p.DefaultBranch = "master"
	}
	if c.PullMethod == PullMethodSSH {
		p.CloneURL = project.SSHURLToRepo
	} else {
		p.CloneURL = project.HTTPURLToRepo
	}
	return p
}
