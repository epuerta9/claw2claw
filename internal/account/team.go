package account

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Team represents a team from the API
type Team struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	Plan      string    `json:"plan"`
}

// TeamMember represents a team member
type TeamMember struct {
	TeamID   string    `json:"team_id"`
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
}

// CreateTeamResult is returned by CreateTeam
type CreateTeamResult struct {
	Team      *Team  `json:"team"`
	JoinToken string `json:"join_token"`
}

// JoinTeamResult is returned by JoinTeam
type JoinTeamResult struct {
	Team   *Team `json:"team"`
	Joined bool  `json:"joined"`
}

// CreateTeam creates a new team
func CreateTeam(cfg *Config, name, slug string, members []string) (*CreateTeamResult, error) {
	if !cfg.LoggedIn {
		return nil, fmt.Errorf("not logged in")
	}

	membersJSON, _ := json.Marshal(members)
	body := fmt.Sprintf(`{"name":%q,"slug":%q,"members":%s}`, name, slug, string(membersJSON))

	req, _ := http.NewRequest("POST", cfg.BaseURL+"/api/v1/teams", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil, fmt.Errorf("team slug '%s' is already taken", slug)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create team: %d", resp.StatusCode)
	}

	var result CreateTeamResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// JoinTeam joins a team using a join token
func JoinTeam(cfg *Config, token string) (*JoinTeamResult, error) {
	if !cfg.LoggedIn {
		return nil, fmt.Errorf("not logged in")
	}

	body := fmt.Sprintf(`{"token":%q}`, token)

	req, _ := http.NewRequest("POST", cfg.BaseURL+"/api/v1/teams/join", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != "" {
			return nil, fmt.Errorf("%s", errResp.Error)
		}
		return nil, fmt.Errorf("failed to join team: %d", resp.StatusCode)
	}

	var result JoinTeamResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTeamInfo returns info about the current team
func GetTeamInfo(cfg *Config) (*Team, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return nil, fmt.Errorf("not logged in or team not configured")
	}

	req, _ := http.NewRequest("GET", cfg.BaseURL+"/api/v1/teams/"+cfg.TeamID, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get team: %d", resp.StatusCode)
	}

	var team Team
	if err := json.NewDecoder(resp.Body).Decode(&team); err != nil {
		return nil, err
	}
	return &team, nil
}

// GetTeamMembers returns members of the current team
func GetTeamMembers(cfg *Config) ([]TeamMember, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return nil, fmt.Errorf("not logged in or team not configured")
	}

	req, _ := http.NewRequest("GET", cfg.BaseURL+"/api/v1/teams/"+cfg.TeamID+"/members", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Members []TeamMember `json:"members"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Members, nil
}

// CreateJoinToken generates a new join token for the current team
func CreateJoinToken(cfg *Config) (string, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return "", fmt.Errorf("not logged in or team not configured")
	}

	req, _ := http.NewRequest("POST", cfg.BaseURL+"/api/v1/teams/"+cfg.TeamID+"/tokens",
		strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Token, nil
}
