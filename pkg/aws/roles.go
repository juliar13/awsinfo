package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// 現在のユーザー情報を取得する
func GetCurrentUserName(ctx context.Context) (string, error) {
	// AWSの設定をロード（デフォルトリージョンを設定）
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("ap-northeast-1"))
	if err != nil {
		return "", fmt.Errorf("AWS設定のロード中にエラーが発生しました: %w", err)
	}

	// STSクライアントを作成
	stsClient := sts.NewFromConfig(cfg)

	// 現在のIDを取得
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("呼び出し元のIDを取得中にエラーが発生しました: %w", err)
	}

	// ARNからユーザー名を抽出
	arnParts := strings.Split(*identity.Arn, "/")
	userName := arnParts[len(arnParts)-1]

	return userName, nil
}

// ロール情報を含むアカウント情報を表す構造体
type AccountRoleInfo struct {
	AccountID string
	RoleName  string
}

// スイッチロールの情報を取得する
func GetSwitchRoleInfo(ctx context.Context, userName string) ([]AccountRoleInfo, error) {
	// AWSの設定をロード（デフォルトリージョンを設定）
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("ap-northeast-1"))
	if err != nil {
		return nil, fmt.Errorf("AWS設定のロード中にエラーが発生しました: %w", err)
	}

	// IAMクライアントを作成
	iamClient := iam.NewFromConfig(cfg)

	// ユーザーのグループを取得
	groupsOutput, err := iamClient.ListGroupsForUser(ctx, &iam.ListGroupsForUserInput{
		UserName: &userName,
	})
	if err != nil {
		return nil, fmt.Errorf("ユーザーのグループ一覧取得中にエラーが発生しました: %w", err)
	}

	// スイッチロール情報を保存するスライス
	var switchRoles []AccountRoleInfo

	// ユーザーの権限ポリシーを取得
	policiesOutput, err := iamClient.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{
		UserName: &userName,
	})
	if err != nil {
		return nil, fmt.Errorf("ユーザーのポリシー一覧取得中にエラーが発生しました: %w", err)
	}

	// ユーザーからスイッチロール情報を抽出
	for _, policy := range policiesOutput.AttachedPolicies {
		policyRoles, err := extractRolesFromPolicy(ctx, iamClient, *policy.PolicyArn)
		if err != nil {
			return nil, err
		}
		switchRoles = append(switchRoles, policyRoles...)
	}

	// 各グループからスイッチロール情報を抽出
	for _, group := range groupsOutput.Groups {
		// グループにアタッチされたポリシーを取得
		groupPoliciesOutput, err := iamClient.ListAttachedGroupPolicies(ctx, &iam.ListAttachedGroupPoliciesInput{
			GroupName: group.GroupName,
		})
		if err != nil {
			return nil, fmt.Errorf("グループのポリシー一覧取得中にエラーが発生しました: %w", err)
		}

		// 各ポリシーからスイッチロール情報を抽出
		for _, policy := range groupPoliciesOutput.AttachedPolicies {
			policyRoles, err := extractRolesFromPolicy(ctx, iamClient, *policy.PolicyArn)
			if err != nil {
				return nil, err
			}
			switchRoles = append(switchRoles, policyRoles...)
		}
	}

	return switchRoles, nil
}

// ポリシーからスイッチロール情報を抽出する補助関数
func extractRolesFromPolicy(ctx context.Context, iamClient *iam.Client, policyArn string) ([]AccountRoleInfo, error) {
	// 結果を格納するスライス
	var roles []AccountRoleInfo

	// ポリシーのバージョン情報を取得
	policyOutput, err := iamClient.GetPolicy(ctx, &iam.GetPolicyInput{
		PolicyArn: &policyArn,
	})
	if err != nil {
		return nil, fmt.Errorf("ポリシー情報取得中にエラーが発生しました: %w", err)
	}

	// ポリシードキュメントを取得
	policyVersionOutput, err := iamClient.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
		PolicyArn: &policyArn,
		VersionId: policyOutput.Policy.DefaultVersionId,
	})
	if err != nil {
		return nil, fmt.Errorf("ポリシーバージョン情報取得中にエラーが発生しました: %w", err)
	}

	// TODO: ポリシードキュメントからアカウントとロール情報を抽出
	// この部分は実装が複雑なため、実際のAPIレスポンスを見ながら調整が必要かもしれません
	// デモ用にダミーデータを返します
	// 実際の実装では、*policyVersionOutput.PolicyVersion.Document から
	// "sts:AssumeRole" アクションが許可されているリソースを解析する必要があります

	// ここでは簡易的な実装として、ポリシードキュメントをログに出力するだけにします
	fmt.Printf("ポリシードキュメント: %v\n", *policyVersionOutput.PolicyVersion.Document)

	// ダミーデータのみを返します
	// 実際の実装ではここでポリシードキュメントを解析してアカウントとロール情報を抽出します
	/*
	roles = append(roles, AccountRoleInfo{
		AccountID: "123456789012",
		RoleName:  "ReadOnlySwitchRole",
	})
	roles = append(roles, AccountRoleInfo{
		AccountID: "123456789012",
		RoleName:  "AdminSwitchRole",
	})
	roles = append(roles, AccountRoleInfo{
		AccountID: "123456789013",
		RoleName:  "AdminSwitchRole",
	})
	*/

	return roles, nil
}
