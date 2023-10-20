// Code generated by "go generate"; DO NOT EDIT.

package paths

import "fmt"

func GithubApps() string {
	return "/app/github-apps"
}

func CreateGithubApp() string {
	return "/app/github-apps/create"
}

func NewGithubApp() string {
	return "/app/github-apps/new"
}

func GithubApp(githubApp string) string {
	return fmt.Sprintf("/app/github-apps/%s", githubApp)
}

func EditGithubApp(githubApp string) string {
	return fmt.Sprintf("/app/github-apps/%s/edit", githubApp)
}

func UpdateGithubApp(githubApp string) string {
	return fmt.Sprintf("/app/github-apps/%s/update", githubApp)
}

func DeleteGithubApp(githubApp string) string {
	return fmt.Sprintf("/app/github-apps/%s/delete", githubApp)
}

func ExchangeCodeGithubApp() string {
	return "/app/github-apps/exchange-code"
}

func CompleteGithubApp() string {
	return "/app/github-apps/complete"
}

func DeleteInstallGithubApp(githubApp string) string {
	return fmt.Sprintf("/app/github-apps/%s/delete-install", githubApp)
}