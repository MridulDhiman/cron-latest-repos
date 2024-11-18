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
    Name string
    LastCommitTime time.Time
	Description string
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

    // Get all repositories where the user has pushed in the last 24 hours
    repos, err := getRecentlyActiveRepos(ctx, client, user.GetLogin())
    if err != nil {
        return fmt.Errorf("failed to get active repos: %v", err)
    }

    if len(repos) == 0 || (len(repos) == 1 && repos[0].Name == "MridulDhiman") {
        log.Println("No repository activity in the last 24 hours")
        return nil
    } 

    // Create commit message with the list of active repositories
    message := createCommitMessage(repos)

    // Update the tracking repository
    err = updateTrackingRepo(ctx, client, message)
    if err != nil {
        return fmt.Errorf("failed to update tracking repo: %v", err)
    }

    return nil
}

func getRecentlyActiveRepos(ctx context.Context, client *github.Client, username string) ([]RepoActivity, error) {
    // Time 24 hours ago
    since := time.Now().Add(-24 * time.Hour)
    
    // List all repositories for the user
    opt := &github.RepositoryListOptions{
        ListOptions: github.ListOptions{PerPage: 100},
    }

    var activeRepos []RepoActivity

    for {
        repos, resp, err := client.Repositories.List(ctx, username, opt)
        if err != nil {
            return nil, err
        }

        for _, repo := range repos {
            // Get commits for each repository
            commits, _, err := client.Repositories.ListCommits(ctx, username, repo.GetName(), &github.CommitsListOptions{
                Since: since,
                Author: username,
            })
            
            if err != nil {
                log.Printf("Error getting commits for %s: %v", repo.GetName(), err)
                continue
            }

            if len(commits) > 0 {
				
                activeRepos = append(activeRepos, RepoActivity{
                    Name: repo.GetName(),
                    LastCommitTime: commits[0].Commit.Author.GetDate(),
					Description: repo.GetDescription(),
                })
            }
        }

        if resp.NextPage == 0 {
            break
        }
        opt.Page = resp.NextPage
    }

    return activeRepos, nil
}

func createCommitMessage(repos []RepoActivity) string {
    var sb strings.Builder
    sb.WriteString(`
Currently exploring backend, devops and genai stuff.

repos I'm currently working on:
	`);

	
    for _, repo := range repos {
		if repo.Name != "MridulDhiman" {
			sb.WriteString(fmt.Sprintf("\n- <a href='https://github.com/MridulDhiman/%s'>%s</a>: %s", 
				repo.Name, repo.Name, repo.Description))
		}
    }

	sb.WriteString(`

Other Projects: 
- <a href="https://github.com/MridulDhiman/remote-code-execution-engine">remote-code-execution-engine</a>: multithreaded code execution API in golang.
- <a href="https://github.com/MridulDhiman/goexpress">goexpress</a>: express.js implementation in golang
- <a href="https://github.com/MridulDhiman/aws.tf">aws.tf</a>: Basic AWS terraform scripts
- <a href="https://github.com/MridulDhiman/source-shift">source-shift</a>: Simple Transpiler in JS
- <a href="https://github.com/MridulDhiman/zanpakuto">zanpakuto</a>: Forge templates through scripts
- <a href="https://github.com/MridulDhiman/BBCalendar">bbcalendar</a>: CLI based Calendar application written in C

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