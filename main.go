package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

type RepoActivity struct {
	ID int64
	Name           string
	LastCommitTime time.Time
	Description    string
	OrgName        string // Added to store organization name
}

func init() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("could not load env. variables", err)
	}
}

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("GITHUB_TOKEN environment variable is required")
	}

	if err := trackGitHubActivity(token); err != nil {
		log.Printf("Error tracking GitHub activity: %v", err)
	}
}

func trackGitHubActivity(token string) error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Get authenticated user
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get user: %v", err)
	}

	lastMonthRepos, err := getActiveRepos(ctx, client, user.GetLogin(), 30)
	if err != nil {
		return fmt.Errorf("failed to get last month repos: %v", err)
	}

	recentRepos, err := getActiveRepos(ctx, client, user.GetLogin(), 1)
	if err != nil {
		return fmt.Errorf("failed to get recent repos: %v", err)
	}

	message := createCommitMessage(recentRepos, lastMonthRepos)
	err = updateTrackingRepo(ctx, client, message)
	if err != nil {
		return fmt.Errorf("failed to update tracking repo: %v", err)
	}

	return nil
}

func getActiveRepos(ctx context.Context, client *github.Client, username string, daysAgo int) ([]RepoActivity, error) {
	since := time.Now().AddDate(0, 0, -daysAgo)
	activeRepos := make(map[string]RepoActivity) // Use map to prevent duplicates
	distinctRepos := make(map[int64]bool)

	opt := &github.ListOptions{PerPage: 100}
	
	for {
		events, resp, err := client.Activity.ListEventsPerformedByUser(ctx, username, false, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list events: %v", err)
		}

		for _, event := range events {
			// Skip events older than our time window
			if event.GetCreatedAt().Before(since) {
				continue
			}

			// We're primarily interested in PushEvents and PullRequestEvents
			if event.GetType() != "PushEvent" && event.GetType() != "PullRequestEvent" {
				continue
			}

			repo := event.GetRepo()
			if repo == nil {
				continue
			}

			// Parse owner and repo name from repo.Name (format: "owner/repo")
			parts := strings.Split(repo.GetName(), "/")
			if len(parts) != 2 {
				continue
			}
			owner, repoName := parts[0], parts[1]

			// Skip if we already have this repo
			if _, exists := activeRepos[repo.GetName()]; exists {
				continue
			}

			// Get repository details
			repoDetails, _, err := client.Repositories.Get(ctx, owner, repoName)
			if err != nil {
				log.Printf("Error getting details for %s: %v", repo.GetName(), err)
				continue
			}

			if repoDetails.GetVisibility() == "public" && repoDetails.GetDescription() != "" {
				if _, exists := distinctRepos[repo.GetID()]; exists {
					continue;
				} else {
					distinctRepos[repo.GetID()] = true;
					activeRepos[repo.GetName()] = RepoActivity{
						Name:           repoName,
						LastCommitTime: event.GetCreatedAt(),
						Description:    repoDetails.GetDescription(),
						OrgName:        owner,
					}

				}
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	// Convert map to slice
	result := make([]RepoActivity, 0, len(activeRepos))
	for _, repo := range activeRepos {
		result = append(result, repo)
	}

	return result, nil
}

func createCommitMessage(repos []RepoActivity, lastMonthRepos []RepoActivity) string {
	var sb strings.Builder
	sb.WriteString(`
Currently exploring backend, devops and genai stuff.

Repos I'm currently working on:
	`)

	for _, repo := range repos {
		if repo.OrgName == "MridulDhiman" {
			sb.WriteString(fmt.Sprintf("\n- <a href='https://github.com/MridulDhiman/%s'>%s</a>: %s",
				repo.Name, repo.Name, repo.Description))
		} else {
			sb.WriteString(fmt.Sprintf("\n- <a href='https://github.com/%s/%s'>%s</a>: %s",
				repo.OrgName, repo.Name, repo.Name, repo.Description))
		}
	}

	sb.WriteString(`

Actively Committed Repos(since last month): 
    `)

	for _, repo := range lastMonthRepos {
		if repo.OrgName == "MridulDhiman" {
			sb.WriteString(fmt.Sprintf("\n- <a href='https://github.com/MridulDhiman/%s'>%s</a>: %s",
				repo.Name, repo.Name, repo.Description))
		} else {
			sb.WriteString(fmt.Sprintf("\n- <a href='https://github.com/%s/%s'>%s</a>: %s",
				repo.OrgName, repo.Name, repo.Name, repo.Description))
		}
	}

	sb.WriteString(`

Open Source Contributions:
- <a href="https://github.com/glasskube/glasskube/issues?q=is%3Aissue+assignee%3AMridulDhiman+is%3Aclosed">glasskube</a>

Check out my blogs <a href="https://mridul.bearblog.dev">here</a>.`)

	return sb.String()
}

func updateTrackingRepo(ctx context.Context, client *github.Client, message string) error {
    // Get the main branch's reference
    ref, _, err := client.Git.GetRef(ctx, "MridulDhiman", "MridulDhiman", "refs/heads/main")
    if err != nil {
        return fmt.Errorf("failed to get ref: %v", err)
    }

    // Create a tree with the new file
    tree, _, err := client.Git.CreateTree(ctx, "MridulDhiman", "MridulDhiman", *ref.Object.SHA, []*github.TreeEntry{
        {
            Path:    github.String("README.md"),
            Mode:    github.String("100644"),
            Type:    github.String("blob"),
            Content: github.String(message),
        },
    })
    if err != nil {
        return fmt.Errorf("failed to create tree: %v", err)
    }

    // Get the parent commit
    parent, _, err := client.Git.GetCommit(ctx, "MridulDhiman", "MridulDhiman", *ref.Object.SHA)
    if err != nil {
        return fmt.Errorf("failed to get parent commit: %v", err)
    }

    // Create the commit
    commit, _, err := client.Git.CreateCommit(ctx, "MridulDhiman", "MridulDhiman", &github.Commit{
        Message: github.String("Update repository activity"),
        Tree:    tree,
        Parents: []*github.Commit{parent},
    })
    if err != nil {
        return fmt.Errorf("failed to create commit: %v", err)
    }

    // Update the reference
    ref.Object.SHA = commit.SHA
    _, _, err = client.Git.UpdateRef(ctx, "MridulDhiman", "MridulDhiman", ref, false)
    if err != nil {
        return fmt.Errorf("failed to update ref: %v", err)
    }

    return nil
}