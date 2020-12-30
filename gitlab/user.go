package gitlab

import (
	"fmt"

	"github.com/xanzy/go-gitlab"
)

type UserFetcher interface {
	FetchUser(uid int) (*User, error)
	FetchCurrentUser() (*User, error)
	FetchUserContent(user *User) (*UserContent, error)
}

type UserContent struct {
	Projects map[string]*Project
}

type User struct {
	ID   int
	Name string

	content *UserContent
}

func NewUserFromGitlabUser(user *gitlab.User) User {
	// https://godoc.org/github.com/xanzy/go-gitlab#User
	return User{
		ID:   user.ID,
		Name: user.Username,
	}
}

func (c *gitlabClient) FetchUser(uid int) (*User, error) {
	gitlabUser, _, err := c.client.Users.GetUser(uid)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user with id %v: %v", uid, err)
	}
	user := NewUserFromGitlabUser(gitlabUser)
	return &user, nil
}

func (c *gitlabClient) FetchCurrentUser() (*User, error) {
	gitlabUser, _, err := c.client.Users.CurrentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current user: %v", err)
	}
	user := NewUserFromGitlabUser(gitlabUser)
	return &user, nil
}

func (c *gitlabClient) FetchUserContent(user *User) (*UserContent, error) {
	if user.content != nil {
		return user.content, nil
	}

	content := &UserContent{
		Projects: map[string]*Project{},
	}

	// Fetch the user repositories
	listProjectOpt := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 1000,
		}}
	for {
		gitlabProjects, response, err := c.client.Projects.ListUserProjects(user.ID, listProjectOpt)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch projects in gitlab: %v", err)
		}
		for _, gitlabProject := range gitlabProjects {
			project := NewProjectFromGitlabProject(gitlabProject)
			content.Projects[project.Name] = &project
		}
		if response.CurrentPage >= response.TotalPages {
			break
		}
		// Get the next page
		listProjectOpt.Page = response.NextPage
	}

	user.content = content
	return content, nil
}
