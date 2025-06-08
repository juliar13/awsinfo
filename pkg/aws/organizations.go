package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/organizations/types"
)

// AccountInfo represents information about an AWS account
type AccountInfo struct {
	AccountID    string
	Name         string
	OUName       string
	CreatedDate  time.Time
	Email        string
	Status       string
	Tags         map[string]string
}

// GetOrganizationInfo retrieves information about all accounts in the organization
func GetOrganizationInfo(ctx context.Context) ([]AccountInfo, error) {
	// Load AWS configuration with default region
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("ap-northeast-1"))
	if err != nil {
		return nil, fmt.Errorf("設定の読み込みに失敗しました: %w", err)
	}

	// Create Organizations client
	orgClient := organizations.NewFromConfig(cfg)

	// Get all accounts
	accounts, err := listAllAccounts(ctx, orgClient)
	if err != nil {
		return nil, fmt.Errorf("アカウント一覧の取得に失敗しました: %w", err)
	}

	// Get OU information
	ouMap, err := buildOUMap(ctx, orgClient)
	if err != nil {
		return nil, fmt.Errorf("OU情報の取得に失敗しました: %w", err)
	}

	var accountInfos []AccountInfo
	for _, account := range accounts {
		// Get OU name for this account
		ouName := getOUNameForAccount(ctx, orgClient, *account.Id, ouMap)

		// Get tags for this account
		tags, err := getAccountTags(ctx, orgClient, *account.Id)
		if err != nil {
			// Don't fail for tag errors, just log and continue
			tags = make(map[string]string)
		}

		accountInfo := AccountInfo{
			AccountID:   aws.ToString(account.Id),
			Name:        aws.ToString(account.Name),
			OUName:      ouName,
			CreatedDate: aws.ToTime(account.JoinedTimestamp),
			Email:       aws.ToString(account.Email),
			Status:      string(account.Status),
			Tags:        tags,
		}

		accountInfos = append(accountInfos, accountInfo)
	}

	return accountInfos, nil
}

// listAllAccounts retrieves all accounts in the organization
func listAllAccounts(ctx context.Context, client *organizations.Client) ([]types.Account, error) {
	var allAccounts []types.Account
	var nextToken *string

	for {
		input := &organizations.ListAccountsInput{
			NextToken: nextToken,
		}

		result, err := client.ListAccounts(ctx, input)
		if err != nil {
			return nil, err
		}

		allAccounts = append(allAccounts, result.Accounts...)

		if result.NextToken == nil {
			break
		}
		nextToken = result.NextToken
	}

	return allAccounts, nil
}

// buildOUMap creates a map of OU ID to OU name
func buildOUMap(ctx context.Context, client *organizations.Client) (map[string]string, error) {
	ouMap := make(map[string]string)

	// Get root information first
	roots, err := client.ListRoots(ctx, &organizations.ListRootsInput{})
	if err != nil {
		return nil, err
	}

	if len(roots.Roots) == 0 {
		return ouMap, nil
	}

	rootId := *roots.Roots[0].Id
	ouMap[rootId] = "Root"

	// Recursively get all OUs
	err = getOUsRecursively(ctx, client, rootId, ouMap)
	if err != nil {
		return nil, err
	}

	return ouMap, nil
}

// getOUsRecursively recursively retrieves all OUs
func getOUsRecursively(ctx context.Context, client *organizations.Client, parentId string, ouMap map[string]string) error {
	var nextToken *string

	for {
		input := &organizations.ListOrganizationalUnitsForParentInput{
			ParentId:  aws.String(parentId),
			NextToken: nextToken,
		}

		result, err := client.ListOrganizationalUnitsForParent(ctx, input)
		if err != nil {
			return err
		}

		for _, ou := range result.OrganizationalUnits {
			ouMap[*ou.Id] = *ou.Name

			// Recursively get child OUs
			err = getOUsRecursively(ctx, client, *ou.Id, ouMap)
			if err != nil {
				return err
			}
		}

		if result.NextToken == nil {
			break
		}
		nextToken = result.NextToken
	}

	return nil
}

// getOUNameForAccount gets the OU name for a specific account
func getOUNameForAccount(ctx context.Context, client *organizations.Client, accountId string, ouMap map[string]string) string {
	// Get parents for this account
	input := &organizations.ListParentsInput{
		ChildId: aws.String(accountId),
	}

	result, err := client.ListParents(ctx, input)
	if err != nil || len(result.Parents) == 0 {
		return "Unknown"
	}

	// Get the first parent (accounts typically have one parent)
	parentId := *result.Parents[0].Id
	if ouName, exists := ouMap[parentId]; exists {
		return ouName
	}

	return "Unknown"
}

// getAccountTags retrieves tags for a specific account
func getAccountTags(ctx context.Context, client *organizations.Client, accountId string) (map[string]string, error) {
	input := &organizations.ListTagsForResourceInput{
		ResourceId: aws.String(accountId),
	}

	result, err := client.ListTagsForResource(ctx, input)
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string)
	for _, tag := range result.Tags {
		tags[*tag.Key] = *tag.Value
	}

	return tags, nil
}

// FormatAccountInfoTable formats account information as a table
func FormatAccountInfoTable(accounts []AccountInfo) string {
	if len(accounts) == 0 {
		return "アカウント情報が見つかりません。"
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("%-15s %-25s %-20s %-12s %-30s %-10s %-20s\n",
		"Account ID", "Name", "OU Name", "Created", "Email", "Status", "Tags"))
	sb.WriteString(strings.Repeat("-", 132) + "\n")

	// Data rows
	for _, account := range accounts {
		// Format created date
		createdStr := account.CreatedDate.Format("2006-01-02")

		// Format tags
		var tagPairs []string
		for k, v := range account.Tags {
			tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", k, v))
		}
		tagsStr := strings.Join(tagPairs, ",")
		if len(tagsStr) > 18 {
			tagsStr = tagsStr[:15] + "..."
		}

		// Truncate long fields
		name := account.Name
		if len(name) > 23 {
			name = name[:20] + "..."
		}

		ouName := account.OUName
		if len(ouName) > 18 {
			ouName = ouName[:15] + "..."
		}

		email := account.Email
		if len(email) > 28 {
			email = email[:25] + "..."
		}

		sb.WriteString(fmt.Sprintf("%-15s %-25s %-20s %-12s %-30s %-10s %-20s\n",
			account.AccountID, name, ouName, createdStr, email, account.Status, tagsStr))
	}

	return sb.String()
}