package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"
    "strings"
    
    "github.com/google/go-github/v45/github"
    "github.com/robfig/cron/v3"
    "golang.org/x/oauth2"
)

type RepoActivity struct {
    Name string
    LastCommitTime time.Time
}

func main() {
    // GitHub token should be set as an environment variable
    token := os.Getenv("GITHUB_TOKEN")
    if token == "" {
        log.Fatal("GITHUB_TOKEN environment variable is required")
    }

    c := cron.New()
    
    // Schedule the job to run every 24 hours
    _, err := c.AddFunc("@daily", func() {
        if err := trackGitHubActivity(token); err != nil {
            log.Printf("Error tracking GitHub activity: %v", err)
        }
    })
    
    if err != nil {
        log.Fatal(err)
    }

    c.Start()
    
    // Keep the program running
    select {}
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

    if len(repos) == 0 {
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
    sb.WriteString("Recent repository activity:\n\n")
    
    for _, repo := range repos {
        sb.WriteString(fmt.Sprintf("- %s (Last commit: %s)\n", 
            repo.Name, 
            repo.LastCommitTime.Format("2006-01-02 15:04:05")))
    }
    
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
            Path:    github.String("activity.md"),
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